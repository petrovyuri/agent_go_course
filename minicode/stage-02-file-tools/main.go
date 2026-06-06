// minicode/stage-02-file-tools/main.go

// Mini Code — мини-агент-кодер на Go (Eino + Ollama).
// Модуль 6: работа с файлами. Даём агенту инструменты на чтение
// (read_file, list_dir, grep) и собираем их в граф Eino: модель решает,
// какой инструмент вызвать, узел инструментов его исполняет, а затем модель
// формулирует ответ по результату. Один раунд инструментов на запрос —
// многошаговый ReAct-цикл соберём в модуле 8.
//
// Запуск из папки модуля:
//
//	go mod tidy
//	go run .
package main

import (
	"bufio"   // построчное чтение из терминала
	"context" // контекст: таймаут и отмена
	"fmt"     // ввод-вывод в консоль
	"log"     // log.Fatalf — остановиться с понятной ошибкой
	"os"      // доступ к stdin

	"strings" // обрезка пробелов во вводе

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/components/model"            // ToolCallingChatModel
	"github.com/cloudwego/eino/components/tool"             // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // Graph, ToolsNode, Lambda
	"github.com/cloudwego/eino/schema"                      // Message, ToolInfo
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	systemPrompt  = "Ты Mini Code — помощник-кодер. Для работы с файлами вызывай " +
		"инструменты read_file, list_dir и grep. Отвечай на русском, кратко и по делу."
)

// agent — это скомпилированный граф: на входе история сообщений,
// на выходе — финальное сообщение модели.
type agent = compose.Runnable[[]*schema.Message, *schema.Message]

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev. После этого граф, который мы скомпилируем ниже,
	// будет виден в плагине EinoDev — он подключается к этому процессу.
	// Если плагин не нужен, ошибка не критична: агент работает и без него.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// Собираем агента-граф: модель с инструментами + узлы исполнения.
	miniCode, err := buildAgent(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать агента: %v", err)
	}

	fmt.Println("Mini Code готов. Напишите запрос (exit — выход).")

	// REPL: читаем запросы из терминала и прогоняем каждый через граф.
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break // конец ввода (Ctrl+D)
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" {
			break
		}

		// Стартовая история: системная инструкция + запрос пользователя.
		msgs := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(input),
		}
		answer, err := miniCode.Invoke(ctx, msgs)
		if err != nil {
			fmt.Println("ошибка:", err)
			continue
		}
		fmt.Println(answer.Content)
	}

	fmt.Println("Пока!")
}

// buildAgent создаёт инструменты чтения, привязывает их к модели и собирает
// граф "модель → (инструменты → модель)". Возвращает скомпилированный граф.
func buildAgent(ctx context.Context) (agent, error) {
	// Три инструмента на чтение. Функции и их параметры — в tools.go.
	readTool, err := utils.InferTool("read_file", "Читает текстовый файл по пути", readFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент read_file: %w", err)
	}
	listTool, err := utils.InferTool("list_dir", "Показывает содержимое папки", listDir)
	if err != nil {
		return nil, fmt.Errorf("инструмент list_dir: %w", err)
	}
	grepTool, err := utils.InferTool("grep", "Ищет подстроку в файле", grep)
	if err != nil {
		return nil, fmt.Errorf("инструмент grep: %w", err)
	}
	tools := []tool.BaseTool{readTool, listTool, grepTool}

	// Модель. Выключаем режим "размышлений" (thinking): по умолчанию qwen3.5
	// тратит время на внутренние рассуждения перед каждым ответом. Для вызова
	// инструментов это не нужно — без размышлений ответы заметно быстрее.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	// Собираем описания инструментов и привязываем их к модели.
	infos := make([]*schema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		info, errInfo := t.Info(ctx)
		if errInfo != nil {
			return nil, fmt.Errorf("описание инструмента: %w", errInfo)
		}
		infos = append(infos, info)
	}
	withTools, err := chatModel.WithTools(infos)
	if err != nil {
		return nil, fmt.Errorf("привязка инструментов: %w", err)
	}

	// Узел, который исполнит вызовы инструментов.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		return nil, fmt.Errorf("создание ToolsNode: %w", err)
	}

	return compileGraph(ctx, withTools, toolsNode)
}

// compileGraph собирает граф из узлов-Lambda и компилирует его.
//
// Узлы и поток данных (по графу везде ходит история []*schema.Message):
//
//	START → agent ─┬─(есть вызовы?)→ exec → respond → done_after → END
//	               └─(нет вызовов)──────────────────→ done       → END
func compileGraph(ctx context.Context, withTools model.ToolCallingChatModel, toolsNode *compose.ToolsNode) (agent, error) {
	// Узел "agent": модель смотрит на историю и решает — ответить сразу
	// или попросить вызвать инструмент. Её ответ дописываем в историю.
	agentNode := compose.InvokableLambda(func(ctx context.Context, msgs []*schema.Message) ([]*schema.Message, error) {
		resp, err := withTools.Generate(ctx, msgs)
		if err != nil {
			return nil, fmt.Errorf("генерация (agent): %w", err)
		}
		return append(msgs, resp), nil
	})

	// Узел "exec": исполняет инструменты, которые попросила модель. Вход узла
	// инструментов — последнее сообщение (с ToolCalls), выход — сообщения с
	// результатами; их тоже дописываем в историю.
	execNode := compose.InvokableLambda(func(ctx context.Context, msgs []*schema.Message) ([]*schema.Message, error) {
		last := msgs[len(msgs)-1]
		toolMsgs, err := toolsNode.Invoke(ctx, last)
		if err != nil {
			return nil, fmt.Errorf("исполнение инструментов: %w", err)
		}
		return append(msgs, toolMsgs...), nil
	})

	// Узел "respond": модель видит результаты инструментов и формулирует
	// окончательный ответ пользователю.
	respondNode := compose.InvokableLambda(func(ctx context.Context, msgs []*schema.Message) ([]*schema.Message, error) {
		resp, err := withTools.Generate(ctx, msgs)
		if err != nil {
			return nil, fmt.Errorf("генерация (respond): %w", err)
		}
		return append(msgs, resp), nil
	})

	// Узлы "done"/"done_after": достаём из истории последнее сообщение — это и
	// есть готовый ответ. Их два, потому что в один узел графа нельзя войти из
	// двух разных мест, а финал у нас два: после agent и после respond.
	lastMessage := func(_ context.Context, msgs []*schema.Message) (*schema.Message, error) {
		return msgs[len(msgs)-1], nil
	}

	// Собираем граф. addErr копит первую ошибку конструирования, чтобы не
	// загромождать код проверкой после каждого шага.
	g := compose.NewGraph[[]*schema.Message, *schema.Message]()
	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}

	add(g.AddLambdaNode("agent", agentNode))
	add(g.AddLambdaNode("exec", execNode))
	add(g.AddLambdaNode("respond", respondNode))
	add(g.AddLambdaNode("done", compose.InvokableLambda(lastMessage)))
	add(g.AddLambdaNode("done_after", compose.InvokableLambda(lastMessage)))

	// Ветвление после "agent": есть вызовы инструментов — идём в "exec";
	// нет — ответ уже готов, идём в "done".
	branch := compose.NewGraphBranch(
		func(_ context.Context, msgs []*schema.Message) (string, error) {
			if len(msgs[len(msgs)-1].ToolCalls) > 0 {
				return "exec", nil
			}
			return "done", nil
		},
		map[string]bool{"exec": true, "done": true},
	)

	add(g.AddEdge(compose.START, "agent"))
	add(g.AddBranch("agent", branch))
	add(g.AddEdge("exec", "respond"))
	add(g.AddEdge("respond", "done_after"))
	add(g.AddEdge("done", compose.END))
	add(g.AddEdge("done_after", compose.END))

	if addErr != nil {
		return nil, fmt.Errorf("сборка графа: %w", addErr)
	}

	// Компиляция проверяет согласованность типов на стыках узлов и возвращает
	// готовый к запуску граф.
	miniCode, err := g.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("компиляция графа: %w", err)
	}
	return miniCode, nil
}
