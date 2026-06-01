// Урок 2.3. Опции вызова: температура, top-p, max tokens.
// Один и тот же запрос отправляем с разными опциями и сравниваем ответы.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст: таймаут и отмена запроса
	"fmt"     // вывод в консоль
	"log"     // log.Fatalf — остановиться с понятной ошибкой

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
	"github.com/cloudwego/eino/components/model"            // опции вызова: WithTemperature и др.
	"github.com/cloudwego/eino/schema"                      // Message и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

func main() {
	ctx := context.Background()

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// Один и тот же творческий запрос — посмотрим, как на него влияет температура.
	messages := []*schema.Message{
		schema.SystemMessage("Ты придумываешь названия. Отвечай одним коротким названием, без пояснений."),
		schema.UserMessage("Придумай название для кофейни рядом с IT-офисом."),
	}

	// Низкая температура (0.0) — модель выбирает самый вероятный вариант, ответ предсказуемый.
	cold, err := chatModel.Generate(ctx, messages, model.WithTemperature(0.0))
	if err != nil {
		log.Fatalf("ошибка генерации (cold): %v", err)
	}
	fmt.Printf("temperature=0.0 → %s\n", cold.Content)

	// Высокая температура (1.0) + ограничение длины — ответ разнообразнее и точно короткий.
	hot, err := chatModel.Generate(ctx, messages,
		model.WithTemperature(1.0),
		model.WithMaxTokens(32),
	)
	if err != nil {
		log.Fatalf("ошибка генерации (hot): %v", err)
	}
	fmt.Printf("temperature=1.0 → %s\n", hot.Content)
}
