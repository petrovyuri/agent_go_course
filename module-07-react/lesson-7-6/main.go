// module-07-react/lesson-7-6/main.go

// Урок 7.6. Стриминг внутри ReAct-агента.
// react.Agent умеет не только Generate (вернуть готовый ответ целиком), но и
// Stream — отдавать финальный ответ по кусочкам, как только модель их печатает.
// Внутренние круги с инструментами агент проходит сам; стримится именно текст
// итогового ответа. Читаем StreamReader в цикле Recv до io.EOF и печатаем чанки
// по мере поступления.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст вызова
	"errors"  // errors.Is для io.EOF
	"fmt"     // вывод в консоль
	"io"      // io.EOF — конец потока
	"log"     // log.Fatalf — остановиться с понятной ошибкой

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino/components/tool"             // интерфейс BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNodeConfig
	"github.com/cloudwego/eino/flow/agent/react"            // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                      // Message
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	systemPrompt  = "Ты ассистент-исследователь. Узнавай численность населения " +
		"инструментом population, складывай числа инструментом add. Не считай в уме — " +
		"всегда пользуйся инструментами. Ответь на русском."
)

// countryArg — параметр инструмента population.
type countryArg struct {
	Country string `json:"country" jsonschema:"required" jsonschema_description:"название страны на русском"`
}

// population возвращает население страны в миллионах человек (мини-справочник).
func population(_ context.Context, in countryArg) (int, error) {
	table := map[string]int{
		"Франция": 68, "Германия": 84, "Япония": 125, "Италия": 59, "Испания": 48,
	}
	n, ok := table[in.Country]
	if !ok {
		return 0, fmt.Errorf("нет данных по стране: %s", in.Country)
	}
	return n, nil
}

// addArgs — параметры инструмента add.
type addArgs struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое число"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе число"`
}

// add складывает два числа.
func add(_ context.Context, in addArgs) (int, error) {
	return in.A + in.B, nil
}

func main() {
	ctx := context.Background()

	popTool, err := utils.InferTool("population", "Население страны в миллионах", population)
	if err != nil {
		log.Fatalf("инструмент population: %v", err)
	}
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

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{popTool, addTool}},
	})
	if err != nil {
		log.Fatalf("создание агента: %v", err)
	}

	// Stream вместо Generate: получаем поток кусочков финального ответа.
	stream, err := agent.Stream(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage("Сколько всего людей живёт во Франции и Германии?"),
	})
	if err != nil {
		log.Fatalf("стрим: %v", err)
	}
	defer stream.Close() // поток нужно закрыть ровно один раз

	// Читаем чанки до конца потока и печатаем их сразу, без переноса строки.
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break // поток закончился
		}
		if err != nil {
			// Печатаем ошибку и выходим через return, чтобы defer stream.Close() отработал.
			fmt.Println("\nошибка чтения потока:", err)
			return
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()
}
