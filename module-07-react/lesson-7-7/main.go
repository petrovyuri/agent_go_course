// module-07-react/lesson-7-7/main.go

// Урок 7.7. Практика: ReAct-агент-исследователь.
// Собираем всё вместе: react.Agent с тремя инструментами (население, столица,
// сложение), системный промпт первым сообщением, лимит шагов и EinoDev для
// просмотра графа. Оборачиваем в REPL — задаём вопросы по очереди; на каждый
// агент сам делает столько кругов ReAct, сколько нужно. Памяти между вопросами
// нет (это тема модуля 9) — каждый запрос самостоятелен.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"bufio"   // построчное чтение из терминала
	"context" // контекст вызова
	"fmt"     // вывод в консоль
	"log"     // log.Fatalf/Printf
	"os"      // доступ к stdin
	"strings" // обрезка пробелов

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
	maxStep       = 12                       // лимит шагов ReAct (защита от зацикливания)
	systemPrompt  = "Ты ассистент-исследователь. Узнавай население инструментом " +
		"population, столицы — инструментом capital, складывай числа инструментом add. " +
		"Не считай и не вспоминай в уме — всегда пользуйся инструментами. Ответь на русском."
)

// countryArg — общий параметр инструментов population и capital.
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

// capital возвращает столицу страны (мини-справочник).
func capital(_ context.Context, in countryArg) (string, error) {
	table := map[string]string{
		"Франция": "Париж", "Германия": "Берлин", "Япония": "Токио",
		"Италия": "Рим", "Испания": "Мадрид",
	}
	c, ok := table[in.Country]
	if !ok {
		return "", fmt.Errorf("нет данных по стране: %s", in.Country)
	}
	return c, nil
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

// buildAgent собирает ReAct-агента с тремя инструментами.
func buildAgent(ctx context.Context) (*react.Agent, error) {
	popTool, err := utils.InferTool("population", "Население страны в миллионах", population)
	if err != nil {
		return nil, fmt.Errorf("инструмент population: %w", err)
	}
	capTool, err := utils.InferTool("capital", "Столица страны", capital)
	if err != nil {
		return nil, fmt.Errorf("инструмент capital: %w", err)
	}
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		return nil, fmt.Errorf("инструмент add: %w", err)
	}

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{popTool, capTool, addTool}},
		MaxStep:          maxStep,
	})
}

func main() {
	ctx := context.Background()

	// EinoDev: граф агента виден в плагине, пока работает REPL.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	agent, err := buildAgent(ctx)
	if err != nil {
		log.Fatalf("сборка агента: %v", err)
	}

	fmt.Println("Агент-исследователь готов. Задайте вопрос (exit — выход).")
	fmt.Println("Например: Сколько всего людей живёт во Франции и Германии?")

	// REPL: каждый вопрос — отдельный запуск ReAct-агента.
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "exit" {
			break
		}

		answer, err := agent.Generate(ctx, []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(question),
		})
		if err != nil {
			fmt.Println("ошибка:", err)
			continue
		}
		fmt.Println(answer.Content)
	}

	fmt.Println("Пока!")
}
