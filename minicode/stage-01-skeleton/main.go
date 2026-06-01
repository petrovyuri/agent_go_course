// Mini Code — мини-агент-кодер на Go (Eino + Ollama).
// Модуль 4: каркас. Терминал читает запрос, прогоняет его через граф к модели
// и печатает ответ. В графе уже стоит маршрутизатор с заготовкой под инструменты
// (ветка "useTool" — заглушка; реальные инструменты появятся в модуле 6).
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
	"github.com/cloudwego/eino/compose"                     // оркестрация: Graph
	"github.com/cloudwego/eino/schema"                      // Message и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	systemPrompt  = "Ты Mini Code — дружелюбный помощник-кодер. " +
		"Отвечай кратко и по делу, на русском языке."
)

func main() {
	ctx := context.Background()

	// Собираем агента (граф) один раз — дальше переиспользуем в цикле.
	agent, err := buildAgent(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать агента: %v", err)
	}

	fmt.Println("Mini Code готов. Напишите запрос (exit — выход).")

	// REPL: читаем строки из терминала, пока не получим exit или Ctrl+D.
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

		// Прогоняем запрос через граф и печатаем ответ модели.
		out, err := agent.Invoke(ctx, input)
		if err != nil {
			fmt.Println("ошибка:", err)
			continue
		}
		fmt.Println(out.Content)
	}

	fmt.Println("Пока!")
}

// buildAgent собирает граф Mini Code: маршрутизатор решает, ответить самому или
// (в будущем) позвать инструмент. Сейчас инструментов нет, поэтому ветвление
// всегда ведёт в ветку "respond" — это скелет под модуль 6.
func buildAgent(ctx context.Context) (compose.Runnable[string, *schema.Message], error) {
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	// Вход графа — строка-запрос, выход — ответ модели (*schema.Message).
	g := compose.NewGraph[string, *schema.Message]()

	// router — узел-вход: пока просто пропускает запрос дальше (его читает ветвление).
	_ = g.AddLambdaNode("router", compose.InvokableLambda(func(ctx context.Context, q string) (string, error) {
		return q, nil
	}))

	// respond — собирает сообщения для модели из системного промпта и запроса.
	_ = g.AddLambdaNode("respond", compose.InvokableLambda(func(ctx context.Context, q string) ([]*schema.Message, error) {
		return []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(q),
		}, nil
	}))

	// model — общий узел-модель: принимает сообщения и отвечает.
	_ = g.AddChatModelNode("model", chatModel)

	// useTool — заглушка под инструменты. Пока недостижима (ветвление туда не ведёт),
	// но в модуле 6 здесь будет вызов ToolsNode.
	_ = g.AddLambdaNode("useTool", compose.InvokableLambda(func(ctx context.Context, q string) (*schema.Message, error) {
		return schema.AssistantMessage("Инструменты появятся в модуле 6 — пока я только отвечаю.", nil), nil
	}))

	_ = g.AddEdge(compose.START, "router")

	// Ветвление: пока инструментов нет — всегда отвечаем сами.
	// В модуле 6 здесь появится выбор между "respond" и "useTool".
	branch := compose.NewGraphBranch(
		func(ctx context.Context, q string) (string, error) {
			return "respond", nil
		},
		map[string]bool{"respond": true, "useTool": true},
	)
	_ = g.AddBranch("router", branch)

	_ = g.AddEdge("respond", "model")
	_ = g.AddEdge("model", compose.END)
	_ = g.AddEdge("useTool", compose.END)

	return g.Compile(ctx)
}
