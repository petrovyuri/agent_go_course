// module-07-react/lesson-7-2/main.go

// Урок 7.2. ReAct-цикл вручную.
// Собираем цикл reasoning -> action -> observation на обычном for: спрашиваем
// модель; если она просит инструменты — исполняем их и возвращаем результат;
// повторяем, пока модель не ответит без вызовов. Это и есть ReAct, только
// условие и повтор написаны на Go. В настоящий граф с циклом его завернёт
// react.Agent (урок 7.4) — графу для накопления истории нужна память
// состояния, а её разберём в модуле 9.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст вызова
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
	maxSteps      = 8                        // страховка от бесконечного цикла
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

	// Два инструмента: справка о населении и сложение.
	popTool, err := utils.InferTool("population", "Население страны в миллионах", population)
	if err != nil {
		log.Fatalf("инструмент population: %v", err)
	}
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("инструмент add: %v", err)
	}
	tools := []tool.BaseTool{popTool, addTool}

	// Модель с привязанными инструментами.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false}, // без "размышлений" — быстрее
	})
	if err != nil {
		log.Fatalf("создание ChatModel: %v", err)
	}
	popInfo, _ := popTool.Info(ctx)
	addInfo, _ := addTool.Info(ctx)
	withTools, err := chatModel.WithTools([]*schema.ToolInfo{popInfo, addInfo})
	if err != nil {
		log.Fatalf("привязка инструментов: %v", err)
	}

	// Узел, который исполняет вызовы инструментов.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		log.Fatalf("создание ToolsNode: %v", err)
	}

	// Стартовая история: системная инструкция + задача пользователя.
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage("Сколько всего людей живёт во Франции и Германии вместе?"),
	}

	// ReAct-цикл: reasoning (Generate) -> action (ToolsNode) -> observation (результат) -> снова.
	for step := 1; step <= maxSteps; step++ {
		resp, err := withTools.Generate(ctx, msgs)
		if err != nil {
			log.Fatalf("генерация: %v", err)
		}
		msgs = append(msgs, resp)

		// Нет вызовов — модель сформулировала ответ, выходим из цикла.
		if len(resp.ToolCalls) == 0 {
			fmt.Printf("\nОтвет: %s\n", resp.Content)
			return
		}

		// Иначе исполняем запрошенные инструменты и возвращаем результаты модели.
		for _, tc := range resp.ToolCalls {
			fmt.Printf("шаг %d: модель просит %s(%s)\n", step, tc.Function.Name, tc.Function.Arguments)
		}
		toolMsgs, err := toolsNode.Invoke(ctx, resp)
		if err != nil {
			log.Fatalf("исполнение инструментов: %v", err)
		}
		for _, m := range toolMsgs {
			fmt.Printf("        наблюдение: %s\n", m.Content)
		}
		msgs = append(msgs, toolMsgs...)
	}

	log.Fatalf("достигнут лимит шагов (%d)", maxSteps)
}
