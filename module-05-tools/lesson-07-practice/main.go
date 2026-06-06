// Урок 5.7. Практика: агент с калькулятором и словарём.
// Собираем маленького агента с тремя инструментами: сложение, умножение и
// поиск по словарю терминов. Модель сама решает, какие инструменты звать.
// Цикл ограничен по числу шагов — полноценный ReAct разберём в модуле 7.
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
	"strings" // нормализация термина

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino/components/tool"             // интерфейс BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNode
	"github.com/cloudwego/eino/schema"                      // Message, ToolInfo
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	maxSteps      = 5                        // защита от бесконечного цикла
)

// glossary — мини-словарь терминов для инструмента define.
var glossary = map[string]string{
	"горутина":  "лёгкий поток выполнения, которым управляет рантайм Go",
	"канал":     "типизированная труба для обмена данными между горутинами",
	"интерфейс": "набор сигнатур методов; тип реализует его неявно",
}

// twoInts — общие параметры арифметических инструментов add и multiply.
type twoInts struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое число"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе число"`
}

// defineArgs — параметр инструмента define: искомый термин.
type defineArgs struct {
	Term string `json:"term" jsonschema:"required" jsonschema_description:"термин для поиска в словаре"`
}

// add складывает два числа.
func add(_ context.Context, in twoInts) (int, error) {
	return in.A + in.B, nil
}

// multiply перемножает два числа.
func multiply(_ context.Context, in twoInts) (int, error) {
	return in.A * in.B, nil
}

// define ищет термин в словаре и возвращает его определение.
func define(_ context.Context, in defineArgs) (string, error) {
	if d, ok := glossary[strings.ToLower(strings.TrimSpace(in.Term))]; ok {
		return d, nil
	}
	return "термин не найден в словаре", nil
}

func main() {
	ctx := context.Background()

	// Создаём три инструмента: сложение, умножение и поиск по словарю.
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		log.Fatalf("инструмент add: %v", err)
	}
	mulTool, err := utils.InferTool("multiply", "Перемножает два целых числа", multiply)
	if err != nil {
		log.Fatalf("инструмент multiply: %v", err)
	}
	defineTool, err := utils.InferTool("define", "Возвращает определение Go-термина из словаря", define)
	if err != nil {
		log.Fatalf("инструмент define: %v", err)
	}
	tools := []tool.BaseTool{addTool, mulTool, defineTool}

	// Создаём модель.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("создание ChatModel: %v", err)
	}

	// Собираем описания всех инструментов и привязываем их к модели.
	infos := make([]*schema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		info, errInfo := t.Info(ctx)
		if errInfo != nil {
			log.Fatalf("описание инструмента: %v", errInfo)
		}
		infos = append(infos, info)
	}
	withTools, err := chatModel.WithTools(infos)
	if err != nil {
		log.Fatalf("привязка инструментов: %v", err)
	}

	// Один узел исполняет все три инструмента.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: tools})
	if err != nil {
		log.Fatalf("создание ToolsNode: %v", err)
	}

	msgs := []*schema.Message{
		schema.SystemMessage("Ты ассистент. Используй инструменты для арифметики и определений терминов."),
		schema.UserMessage("Что такое горутина? И сколько будет 6 умножить на 7?"),
	}

	// Простой цикл: пока модель просит инструменты — исполняем и отдаём ей результат.
	for step := 0; step < maxSteps; step++ {
		resp, errGen := withTools.Generate(ctx, msgs)
		if errGen != nil {
			log.Fatalf("ошибка генерации: %v", errGen)
		}
		msgs = append(msgs, resp)

		if len(resp.ToolCalls) == 0 {
			fmt.Println(resp.Content) // финальный ответ
			return
		}

		toolMsgs, errTool := toolsNode.Invoke(ctx, resp)
		if errTool != nil {
			log.Fatalf("исполнение инструментов: %v", errTool)
		}
		msgs = append(msgs, toolMsgs...)
	}

	fmt.Println("достигнут лимит шагов")
}
