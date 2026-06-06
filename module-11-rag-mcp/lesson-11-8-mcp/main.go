// module-11-rag-mcp/lesson-11-8-mcp/main.go

// Урок 11.8. MCP-сервер как инструменты агента.
//
// MCP (Model Context Protocol) — открытый протокол: внешний сервер отдаёт набор
// инструментов, а агент их вызывает, не зная, как они устроены внутри. Здесь мы:
//  1. поднимаем небольшой MCP-сервер со складским инструментом stock;
//  2. подключаемся к нему in-process клиентом (без отдельного процесса);
//  3. через mcp.GetTools превращаем инструменты сервера в инструменты Eino;
//  4. отдаём их react.Agent — и он сам вызывает stock, отвечая на вопрос.
//
// Так же подключают и настоящие MCP-серверы (по stdio или HTTP) — меняется только
// конструктор клиента, остальной код тот же.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделью qwen3.5. Пока процесс работает, граф агента
// виден в EinoDev (devops.Init).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	chatollama "github.com/cloudwego/eino-ext/components/model/ollama"
	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp" // мост MCP -> инструменты Eino
	"github.com/cloudwego/eino-ext/devops"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	chatModelName = "qwen3.5"                // модель с поддержкой инструментов
	maxStep       = 8                        // лимит шагов ReAct
)

// stockReply — бизнес-логика складского инструмента: остаток товара по названию.
// Вынесена отдельно, чтобы её можно было проверить тестом без сети.
func stockReply(item string) string {
	stock := map[string]int{"гайки": 42, "болты": 17, "шайбы": 100}
	n, ok := stock[strings.ToLower(strings.TrimSpace(item))]
	if !ok {
		return "Товар не найден на складе."
	}
	return "На складе " + strconv.Itoa(n) + " шт.: " + item
}

// newMCPClient поднимает MCP-сервер со складским инструментом и подключает к нему
// in-process клиент. Возвращает уже инициализированный клиент.
func newMCPClient(ctx context.Context) (*client.Client, error) {
	s := server.NewMCPServer("warehouse", "1.0.0")
	s.AddTool(
		mcp.NewTool("stock",
			mcp.WithDescription("Возвращает остаток товара на складе по его названию"),
			mcp.WithString("item", mcp.Description("название товара, например: гайки"), mcp.Required()),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText(stockReply(req.GetString("item", ""))), nil
		},
	)

	cli, err := client.NewInProcessClient(s)
	if err != nil {
		return nil, fmt.Errorf("создание MCP-клиента: %w", err)
	}
	if err := cli.Start(ctx); err != nil {
		return nil, fmt.Errorf("запуск MCP-клиента: %w", err)
	}
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		return nil, fmt.Errorf("инициализация MCP: %w", err)
	}
	return cli, nil
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev — граф агента виден, пока работает процесс.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// 1-3. Поднимаем MCP-сервер, клиент и берём инструменты как инструменты Eino.
	cli, err := newMCPClient(ctx)
	if err != nil {
		log.Fatalf("не удалось подготовить MCP: %v", err)
	}
	tools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: cli})
	if err != nil {
		log.Fatalf("не удалось получить инструменты MCP: %v", err)
	}
	log.Printf("Из MCP получено инструментов: %d", len(tools))

	// 4. Собираем ReAct-агента и отдаём ему инструменты MCP.
	chatModel, err := chatollama.NewChatModel(ctx, &chatollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    chatModelName,
		Thinking: &chatollama.ThinkValue{Value: false},
	})
	if err != nil {
		log.Fatalf("не удалось создать чат-модель: %v", err)
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
		MaxStep:          maxStep,
	})
	if err != nil {
		log.Fatalf("не удалось собрать агента: %v", err)
	}

	question := "Сколько на складе гаек и болтов?"
	msgs := []*schema.Message{
		schema.SystemMessage("Ты складской помощник. Остатки узнавай только через инструмент stock. Отвечай на русском кратко."),
		schema.UserMessage(question),
	}
	answer, err := agent.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("ошибка агента: %v", err)
	}

	fmt.Println("Вопрос:", question)
	fmt.Println("Ответ: ", answer.Content)

	// Держим процесс, чтобы успеть посмотреть граф агента в EinoDev. Ctrl+C — выход.
	log.Println("Готово. Откройте EinoDev (адрес в строке start debug http server). Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
