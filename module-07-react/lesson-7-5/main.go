// module-07-react/lesson-7-5/main.go

// Урок 7.5. Лимит шагов и защита от бесконечного цикла.
// ReAct-цикл может не сойтись: модель будет звать инструменты бесконечно. И в
// ручном цикле (maxSteps в for), и в react.Agent есть страховка — лимит шагов.
// У react.Agent это поле MaxStep (по умолчанию число узлов + 10). Один круг
// ReAct = два шага (ChatModel + ToolsNode), поэтому MaxStep напрямую ограничивает
// число кругов. Поставим маленький лимит и увидим, как агент останавливается,
// не дойдя до ответа.
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

	// MaxStep = 2 — всего один круг ReAct. Для задачи нужно больше, поэтому
	// агент упрётся в лимит и не успеет посчитать сумму.
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{popTool, addTool}},
		MaxStep:          2, // нарочно маленький лимит для демонстрации
	})
	if err != nil {
		log.Fatalf("создание агента: %v", err)
	}

	answer, err := agent.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage("Сколько всего людей живёт во Франции и Германии?"),
	})
	if err != nil {
		// Лимит шагов превышен — агент остановлен. Это и есть защита от зацикливания.
		fmt.Println("сработал лимит шагов:", err)
		return
	}
	fmt.Println(answer.Content)
}
