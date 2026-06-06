// Урок 5.5. Связка ChatModel ↔ Tools: ToolCall и ToolMessage.
// Полный раунд-трип: модель просит инструмент → исполняем его → возвращаем
// результат модели → она отвечает по-человечески.
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

// addArgs — параметры инструмента. Теги задают JSON-схему, которую увидит модель.
type addArgs struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое слагаемое"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе слагаемое"`
}

// add — функция-инструмент, вынесенная из main.
func add(_ context.Context, in addArgs) (int, error) {
	return in.A + in.B, nil
}

func main() {
	ctx := context.Background()

	// 1. Инструмент: обычная Go-функция, обёрнутая в InvokableTool.
	//    InferTool выводит схему параметров из тегов структуры addArgs.
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("создание инструмента: %v", err)
	}

	// 2. Модель и привязка инструмента. WithTools не меняет базовую модель,
	//    а возвращает новую с прикреплёнными инструментами.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("создание ChatModel: %v", err)
	}

	info, err := addTool.Info(ctx)
	if err != nil {
		log.Fatalf("описание инструмента: %v", err)
	}
	withTools, err := chatModel.WithTools([]*schema.ToolInfo{info})
	if err != nil {
		log.Fatalf("привязка инструментов: %v", err)
	}

	// 3. Узел исполнения инструментов.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{addTool},
	})
	if err != nil {
		log.Fatalf("создание ToolsNode: %v", err)
	}

	// 4. Спрашиваем модель. Она может ответить сама или попросить инструмент.
	msgs := []*schema.Message{
		schema.SystemMessage("Ты ассистент. Для арифметики вызывай инструмент add."),
		schema.UserMessage("Сколько будет 2 + 3?"),
	}

	resp, err := withTools.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("ошибка генерации: %v", err)
	}

	if len(resp.ToolCalls) == 0 {
		fmt.Println(resp.Content) // модель решила ответить без инструмента
		return
	}

	// 5. Модель попросила инструмент: добавляем её сообщение и исполняем вызовы.
	//    ToolsNode принимает сообщение с ToolCalls и возвращает ToolMessage'ы.
	msgs = append(msgs, resp)
	toolMsgs, err := toolsNode.Invoke(ctx, resp)
	if err != nil {
		log.Fatalf("исполнение инструментов: %v", err)
	}
	msgs = append(msgs, toolMsgs...)

	// 6. Финальный ответ — модель уже видит результат инструмента.
	final, err := withTools.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("финальная генерация: %v", err)
	}
	fmt.Println(final.Content)
}
