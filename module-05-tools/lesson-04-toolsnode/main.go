// Урок 5.4. ToolsNode: узел исполнения инструментов в графе.
// ToolsNode принимает сообщение ассистента с вызовами (ToolCalls) и исполняет их,
// возвращая по сообщению-результату (ToolMessage) на каждый вызов.
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

	"github.com/cloudwego/eino/components/tool"       // интерфейс BaseTool
	"github.com/cloudwego/eino/components/tool/utils" // конструкторы инструментов
	"github.com/cloudwego/eino/compose"               // ToolsNode
	"github.com/cloudwego/eino/schema"                // Message, ToolCall
)

// addArgs — параметры инструмента add.
type addArgs struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое слагаемое"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе слагаемое"`
}

// add — функция-инструмент (вынесена отдельно).
func add(_ context.Context, in addArgs) (int, error) {
	return in.A + in.B, nil
}

func main() {
	ctx := context.Background()

	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("InferTool: %v", err)
	}

	// ToolsNode — узел, который умеет исполнять переданные ему инструменты.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{addTool},
	})
	if err != nil {
		log.Fatalf("создание ToolsNode: %v", err)
	}

	// Обычно такое сообщение приходит ОТ модели. Здесь соберём его вручную,
	// чтобы посмотреть на ToolsNode без модели: ассистент просит вызвать add.
	assistant := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{
			{
				ID:       "call_1",
				Type:     "function",
				Function: schema.FunctionCall{Name: "add", Arguments: `{"a":2,"b":3}`},
			},
		},
	}

	// ToolsNode исполняет каждый вызов и возвращает ToolMessage'ы — по одному на вызов,
	// в том же порядке. ToolCallID связывает результат с конкретным вызовом.
	toolMsgs, err := toolsNode.Invoke(ctx, assistant)
	if err != nil {
		log.Fatalf("исполнение инструментов: %v", err)
	}

	for _, m := range toolMsgs {
		fmt.Printf("инструмент (вызов %s) вернул: %s\n", m.ToolCallID, m.Content)
	}
}
