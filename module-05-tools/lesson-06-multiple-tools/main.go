// Урок 5.6. Несколько инструментов и выбор моделью нужного.
// Привязываем к модели два инструмента (сложение и умножение). Модель сама
// выбирает подходящий по смыслу запроса.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст
	"fmt"     // вывод в консоль
	"log"     // log.Fatalf — остановиться с понятной ошибкой

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino/components/tool"             // интерфейс BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNode
	"github.com/cloudwego/eino/schema"                      // Message, ToolInfo
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

// twoInts — общие параметры для обоих арифметических инструментов.
type twoInts struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое число"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе число"`
}

// add складывает два числа.
func add(_ context.Context, in twoInts) (int, error) {
	return in.A + in.B, nil
}

// multiply перемножает два числа.
func multiply(_ context.Context, in twoInts) (int, error) {
	return in.A * in.B, nil
}

func main() {
	ctx := context.Background()

	// Два инструмента. Модель выберет нужный по описанию (Desc).
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("инструмент add: %v", err)
	}
	mulTool, err := utils.InferTool("multiply", "Перемножает два целых числа", multiply)
	if err != nil {
		log.Fatalf("инструмент multiply: %v", err)
	}

	// Создаём модель, к которой привяжем инструменты.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("создание ChatModel: %v", err)
	}

	// Описания обоих инструментов отдаём модели разом.
	addInfo, _ := addTool.Info(ctx)
	mulInfo, _ := mulTool.Info(ctx)
	withTools, err := chatModel.WithTools([]*schema.ToolInfo{addInfo, mulInfo})
	if err != nil {
		log.Fatalf("привязка инструментов: %v", err)
	}

	// Один ToolsNode исполняет оба инструмента.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{addTool, mulTool},
	})
	if err != nil {
		log.Fatalf("создание ToolsNode: %v", err)
	}

	msgs := []*schema.Message{
		schema.SystemMessage("Ты ассистент. Для арифметики вызывай подходящий инструмент."),
		schema.UserMessage("Сколько будет 6 умножить на 7?"),
	}

	resp, err := withTools.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("ошибка генерации: %v", err)
	}

	if len(resp.ToolCalls) == 0 {
		fmt.Println(resp.Content)
		return
	}

	// Покажем, какой инструмент выбрала модель.
	for _, tc := range resp.ToolCalls {
		fmt.Printf("модель выбрала инструмент: %s с аргументами %s\n", tc.Function.Name, tc.Function.Arguments)
	}

	msgs = append(msgs, resp)
	toolMsgs, err := toolsNode.Invoke(ctx, resp)
	if err != nil {
		log.Fatalf("исполнение инструментов: %v", err)
	}
	msgs = append(msgs, toolMsgs...)

	final, err := withTools.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("финальная генерация: %v", err)
	}
	fmt.Println(final.Content)
}
