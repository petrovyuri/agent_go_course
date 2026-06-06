// module-07-react/lesson-7-4/main.go

// Урок 7.4. Готовый react.Agent.
// Тот же ReAct-цикл, что мы писали руками, в Eino уже есть из коробки:
// react.NewAgent собирает граф ChatModel -> (есть вызовы? -> ToolsNode ->
// ChatModel) и крутит его сам, пока модель не ответит без вызовов. Нам остаётся
// дать модель и список инструментов. Плюс поднимаем devops.Init — и граф агента
// (с настоящим циклом) виден в плагине EinoDev.
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
	"github.com/cloudwego/eino-ext/devops"                  // сервер EinoDev
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

	// Поднимаем EinoDev: граф react.Agent будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// Инструменты.
	popTool, err := utils.InferTool("population", "Население страны в миллионах", population)
	if err != nil {
		log.Fatalf("инструмент population: %v", err)
	}
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("инструмент add: %v", err)
	}

	// Модель с поддержкой вызова инструментов.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		log.Fatalf("создание ChatModel: %v", err)
	}

	// Собираем ReAct-агента: модель + инструменты. Цикл агент крутит сам.
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{popTool, addTool}},
	})
	if err != nil {
		log.Fatalf("создание агента: %v", err)
	}

	// Запрос отдаём целиком (системный промпт — первым сообщением). Агент сам
	// сделает столько кругов, сколько нужно.
	answer, err := agent.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage("Сколько всего людей живёт во Франции и Германии?"),
	})
	if err != nil {
		log.Fatalf("генерация: %v", err)
	}
	fmt.Println(answer.Content)
}
