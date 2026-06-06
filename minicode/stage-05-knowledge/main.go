// minicode/stage-05-knowledge/main.go

// Mini Code — мини-агент-кодер на Go (Eino + Ollama). Этап 5: знание проекта.
// Берём агента из этапа 4 (react.Agent с памятью, правкой и подтверждением) и
// учим его видеть весь проект через RAG:
//   - при старте индексируем кодовую базу (index.go): файлы режутся на куски,
//     каждый кусок превращается в вектор эмбеддером Ollama (как в модуле 11);
//   - инструмент search_code ищет по смыслу куски, похожие на запрос, — это RAG
//     внутри агента (retrieve перед ответом), в дополнение к буквальному grep.
//
// Граф агента по-прежнему виден в EinoDev (devops.Init).
//
// Запуск из папки модуля:
//
//	go mod tidy
//	go run .
package main

import (
	"bufio"         // чтение из терминала
	"context"       // контекст и таймауты
	"encoding/json" // сохранение/загрузка сессии
	"fmt"           // ввод-вывод в консоль
	"log"           // log.Fatalf/Printf
	"os"            // stdin и файл сессии
	"strings"       // обработка ввода
	"time"          // таймаут хода

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/components/tool"             // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNodeConfig
	"github.com/cloudwego/eino/flow/agent/react"            // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                      // Message
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	sessionFile   = "session.json"           // файл памяти сессии
	turnTimeout   = 5 * time.Minute          // таймаут на один ход агента
	maxStep       = 16                       // лимит шагов ReAct (защита от зацикливания)
	systemPrompt  = "Ты Mini Code — помощник-кодер, который знает этот проект. " +
		"Поиск по коду: search_code (по смыслу) и grep (буквальный). Чтение: read_file, list_dir. " +
		"Правка: write_file, edit_file. Команды: run_command (только go). Внешние инструменты MCP: " +
		"git_log (история коммитов), project_tree (дерево проекта). Если вопрос про проект — сначала " +
		"найди нужный код через search_code, потом отвечай по нему. Действуй по шагам; перед изменением " +
		"коротко скажи, что собираешься сделать. Отвечай на русском, кратко."
)

// reader — общий ввод из терминала: им пользуется и REPL, и подтверждение в
// инструментах, поэтому буфер ввода один на всех.
var reader = bufio.NewReader(os.Stdin)

// readLine читает одну строку из общего ввода (без хвостовых пробелов).
func readLine() (string, bool) {
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", false // конец ввода
	}
	return strings.TrimSpace(line), true
}

// confirm спрашивает подтверждение перед опасным действием. Это переменная —
// так её удобно подменить в тестах.
var confirm = func(prompt string) bool {
	fmt.Print("Mini Code просит подтверждение — " + prompt + " — выполнить? (y/n): ")
	line, ok := readLine()
	if !ok {
		return false
	}
	ans := strings.ToLower(line)
	return ans == "y" || ans == "yes" || ans == "да"
}

// buildAgent собирает ReAct-агента: чтение, семантический поиск, правка, команды
// и инструменты внешнего MCP-сервера (extraTools).
func buildAgent(ctx context.Context, idx *projectIndex, extraTools []tool.BaseTool) (*react.Agent, error) {
	// Модель. Размышления выключаем — без них ответы быстрее (как в модуле 8).
	// Её же передаём в search_code для переписывания запроса (agentic RAG, см. agentic.go).
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	readTool, err := utils.InferTool("read_file", "Читает текстовый файл по пути", readFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент read_file: %w", err)
	}
	listTool, err := utils.InferTool("list_dir", "Показывает содержимое папки", listDir)
	if err != nil {
		return nil, fmt.Errorf("инструмент list_dir: %w", err)
	}
	grepTool, err := utils.InferTool("grep", "Ищет подстроку в файле или во всех файлах папки", grep)
	if err != nil {
		return nil, fmt.Errorf("инструмент grep: %w", err)
	}
	searchTool, err := utils.InferTool("search_code", "Семантический поиск по коду проекта: ищет куски, похожие по смыслу", makeSearchCode(idx, chatModel))
	if err != nil {
		return nil, fmt.Errorf("инструмент search_code: %w", err)
	}
	writeTool, err := utils.InferTool("write_file", "Создаёт или перезаписывает файл (с подтверждением)", writeFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент write_file: %w", err)
	}
	editTool, err := utils.InferTool("edit_file", "Заменяет подстроку в файле (с подтверждением)", editFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент edit_file: %w", err)
	}
	cmdTool, err := utils.InferTool("run_command", "Запускает команду go (с подтверждением)", runCommand)
	if err != nil {
		return nil, fmt.Errorf("инструмент run_command: %w", err)
	}

	// ReAct-агент: инструменты этапа 4 + search_code (RAG по проекту) + инструменты MCP.
	tools := []tool.BaseTool{readTool, listTool, grepTool, searchTool, writeTool, editTool, cmdTool}
	tools = append(tools, extraTools...)
	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
		MaxStep:          maxStep,
	})
}

// loadSession читает историю сессии с диска или начинает новую с системным промптом.
func loadSession() []*schema.Message {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return []*schema.Message{schema.SystemMessage(systemPrompt)}
	}
	var msgs []*schema.Message
	if err := json.Unmarshal(data, &msgs); err != nil || len(msgs) == 0 {
		return []*schema.Message{schema.SystemMessage(systemPrompt)}
	}
	return msgs
}

// saveSession сохраняет историю сессии на диск.
func saveSession(msgs []*schema.Message) {
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		fmt.Println("не удалось сериализовать сессию:", err)
		return
	}
	if err := os.WriteFile(sessionFile, data, 0o600); err != nil {
		fmt.Println("не удалось сохранить сессию:", err)
	}
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev — граф ReAct-агента виден в плагине, пока работает REPL.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// Знание проекта: индексируем кодовую базу текущей папки.
	idx, err := newProjectIndex(ctx, ollamaBaseURL)
	if err != nil {
		log.Fatalf("не удалось создать индекс: %v", err)
	}
	fmt.Println("Индексирую проект...")
	n, err := idx.indexProject(ctx, ".")
	if err != nil {
		log.Fatalf("не удалось проиндексировать проект: %v", err)
	}
	fmt.Printf("Готово: проиндексировано кусков кода — %d.\n", n)

	// Подключаем внешний MCP-сервер (git_log, project_tree). Если не вышло —
	// работаем без него: остальные инструменты на месте.
	mcpTools, mcpCli, err := connectProjectMCP(ctx)
	if err != nil {
		log.Printf("MCP-сервер не подключён: %v", err)
	} else {
		defer mcpCli.Close()
		fmt.Printf("MCP подключён: получено инструментов — %d.\n", len(mcpTools))
	}

	agent, err := buildAgent(ctx, idx, mcpTools)
	if err != nil {
		log.Printf("не удалось собрать агента: %v", err)
		return // не Fatalf: дать отложенному mcpCli.Close() отработать
	}

	msgs := loadSession()
	fmt.Println("Mini Code готов и знает проект. Помнит сессию между запусками.")
	fmt.Println("Команды: exit — выход (сессия сохранится), /reset — забыть историю.")

	// REPL: на каждый запрос прогоняем агента по всей истории (это и есть память).
	for {
		fmt.Print("> ")
		input, ok := readLine()
		if !ok {
			break // конец ввода (Ctrl+D)
		}
		switch input {
		case "":
			continue
		case "exit":
			saveSession(msgs)
			fmt.Println("Пока! Сессия сохранена.")
			return
		case "/reset":
			msgs = []*schema.Message{schema.SystemMessage(systemPrompt)}
			saveSession(msgs)
			fmt.Println("История очищена.")
			continue
		}

		msgs = append(msgs, schema.UserMessage(input))

		// Таймаут на ход: если модель или команда зависнут, ход прервётся.
		genCtx, cancel := context.WithTimeout(ctx, turnTimeout)
		answer, err := agent.Generate(genCtx, msgs)
		cancel()
		if err != nil {
			fmt.Println("ошибка:", err)
			continue
		}
		msgs = append(msgs, answer)
		fmt.Println(answer.Content)
		saveSession(msgs)
	}
}
