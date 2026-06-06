// module-07-react/lesson-7-3/main.go

// Урок 7.3. Условие выхода из ReAct-цикла.
// Выносим цикл в функцию runReAct. Главное в ней — условие выхода: как только
// модель отвечает БЕЗ вызовов инструментов (len(ToolCalls) == 0), задача решена
// и мы возвращаем текст. Если вызовы есть — исполняем и идём на новый круг.
// Проверяем на двух запросах: многошаговом (нужны инструменты) и простом
// приветствии (инструменты не нужны — выходим сразу).
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
	"github.com/cloudwego/eino/components/model"            // ToolCallingChatModel
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

// runReAct прогоняет один вопрос через ReAct-цикл и возвращает ответ модели.
// Условие выхода — ответ без вызовов инструментов.
func runReAct(ctx context.Context, m model.ToolCallingChatModel, tn *compose.ToolsNode, question string) (string, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(question),
	}

	for step := 0; step < maxSteps; step++ {
		resp, err := m.Generate(ctx, msgs)
		if err != nil {
			return "", fmt.Errorf("генерация: %w", err)
		}
		msgs = append(msgs, resp)

		// Условие выхода: модель ответила без вызовов — задача решена.
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		// Иначе исполняем инструменты и продолжаем цикл.
		toolMsgs, err := tn.Invoke(ctx, resp)
		if err != nil {
			return "", fmt.Errorf("исполнение инструментов: %w", err)
		}
		msgs = append(msgs, toolMsgs...)
	}

	return "", fmt.Errorf("достигнут лимит шагов (%d)", maxSteps)
}

func main() {
	ctx := context.Background()

	// Инструменты.
	popTool, err := utils.InferTool("population", "Население страны в миллионах", population)
	if err != nil {
		log.Fatalf("инструмент population: %v", err)
	}
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("инструмент add: %v", err)
	}
	tools := []tool.BaseTool{popTool, addTool}

	// Модель с инструментами.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
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

	// Узел исполнения инструментов.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		log.Fatalf("создание ToolsNode: %v", err)
	}

	// 1) Многошаговый вопрос: цикл сделает несколько кругов с инструментами.
	answer, err := runReAct(ctx, withTools, toolsNode, "Сколько всего людей живёт во Франции и Германии?")
	if err != nil {
		log.Fatalf("вопрос 1: %v", err)
	}
	fmt.Println("вопрос 1:", answer)

	// 2) Простое приветствие: инструменты не нужны — выходим на первом круге.
	answer, err = runReAct(ctx, withTools, toolsNode, "Привет! Кратко представься.")
	if err != nil {
		log.Fatalf("вопрос 2: %v", err)
	}
	fmt.Println("вопрос 2:", answer)
}
