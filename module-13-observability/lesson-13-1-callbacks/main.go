// module-13-observability/lesson-13-1-callbacks/main.go

// Урок 13.1. Callbacks: OnStart / OnEnd / OnError для отладки.
//
// Callbacks — это хуки наблюдаемости Eino: они срабатывают вокруг КАЖДОГО
// компонента (модель, инструмент, узел графа) на старте, на завершении и на
// ошибке. Через них видно, что и в каком порядке делает агент и сколько времени
// уходит на каждый шаг — это база для отладки и трейсинга (LangFuse — урок 13.3).
//
// Здесь собираем простой ReAct-агент с инструментом add и вешаем свой обработчик,
// который печатает трассу: старт/конец каждого компонента и его длительность.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделью qwen3.5.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino/callbacks"                   // хуки наблюдаемости
	"github.com/cloudwego/eino/components/tool"             // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNodeConfig, WithCallbacks
	"github.com/cloudwego/eino/flow/agent"                  // WithComposeOptions
	"github.com/cloudwego/eino/flow/agent/react"            // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                      // Message
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	maxStep       = 8                        // лимит шагов ReAct
)

// startTimeKey — ключ, под которым храним время старта компонента в контексте.
// Контекст из OnStart Eino передаёт в OnEnd того же компонента, поэтому так
// удобно измерять длительность.
type startTimeKey struct{}

// addArgs — параметры инструмента add.
type addArgs struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое слагаемое"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе слагаемое"`
}

// add складывает два целых числа.
func add(_ context.Context, in addArgs) (int, error) {
	return in.A + in.B, nil
}

// traceHandler — обработчик callbacks: печатает старт, конец (с длительностью) и
// ошибку каждого компонента. RunInfo.Type — что это (модель, ToolsNode...),
// RunInfo.Name — имя узла.
func traceHandler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
			if info != nil {
				log.Printf("-> старт  [%s] %s", info.Type, info.Name)
			}
			return context.WithValue(ctx, startTimeKey{}, time.Now())
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackOutput) context.Context {
			var elapsed time.Duration
			if t, ok := ctx.Value(startTimeKey{}).(time.Time); ok {
				elapsed = time.Since(t).Round(time.Millisecond)
			}
			if info != nil {
				log.Printf("<- конец  [%s] %s за %v", info.Type, info.Name, elapsed)
			}
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			if info != nil {
				log.Printf("xx ошибка [%s] %s: %v", info.Type, info.Name, err)
			}
			return ctx
		}).
		Build()
}

func main() {
	ctx := context.Background()

	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("инструмент add: %v", err)
	}

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		log.Fatalf("создание ChatModel: %v", err)
	}

	agentRunner, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{addTool}},
		MaxStep:          maxStep,
	})
	if err != nil {
		log.Fatalf("создание агента: %v", err)
	}

	msgs := []*schema.Message{
		schema.SystemMessage("Ты считаешь только через инструмент add. Отвечай на русском, кратко."),
		schema.UserMessage("Сколько будет 7 + 5, а потом прибавь к результату 100?"),
	}

	// Вешаем обработчик на этот запуск через agent.WithComposeOptions + compose.WithCallbacks.
	answer, err := agentRunner.Generate(ctx, msgs,
		agent.WithComposeOptions(compose.WithCallbacks(traceHandler())))
	if err != nil {
		log.Fatalf("ошибка агента: %v", err)
	}

	fmt.Println("\nОтвет:", answer.Content)
}
