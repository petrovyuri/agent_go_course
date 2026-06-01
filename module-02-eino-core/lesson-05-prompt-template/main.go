// Урок 2.5. Шаблоны промптов: prompt.FromMessages.
// Собираем промпт из шаблона с переменными и местом для истории, затем спрашиваем модель.
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
	"github.com/cloudwego/eino/components/prompt"           // шаблоны промптов
	"github.com/cloudwego/eino/schema"                      // Message, роли, конструкторы
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

	// Шаблон: в фигурных скобках — переменные (формат FString).
	// MessagesPlaceholder — место, куда подставится история диалога.
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("Ты эксперт по {topic}. Отвечай кратко, по делу."),
		schema.MessagesPlaceholder("history", false), // false = история обязательна
		schema.UserMessage("{question}"),
	)

	// Значения подставляются по именам переменных.
	messages, err := template.Format(ctx, map[string]any{
		"topic": "язык Go",
		"history": []*schema.Message{
			schema.UserMessage("Что такое горутина?"),
			schema.AssistantMessage("Горутина — это лёгкий поток, которым управляет рантайм Go.", nil),
		},
		"question": "А как горутины обмениваются данными между собой?",
	})
	if err != nil {
		log.Fatalf("ошибка Format: %v", err)
	}

	// Посмотрим, что получилось после подстановки.
	fmt.Println("Готовый промпт:")
	for _, m := range messages {
		fmt.Printf("  [%s] %s\n", m.Role, m.Content)
	}

	// Отправляем собранные сообщения модели.
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("ошибка генерации: %v", err)
	}
	fmt.Printf("\n  [%s] %s\n", response.Role, response.Content)
}
