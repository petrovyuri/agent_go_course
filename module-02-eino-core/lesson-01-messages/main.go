// Урок 2.1. schema.Message и роли.
// Программа строит диалог из сообщений с разными ролями и отправляет его модели.
// Видно главное: история диалога — это просто срез сообщений []*schema.Message.
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
	"github.com/cloudwego/eino/schema"                      // Message, роли и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

func main() {
	ctx := context.Background()

	// Создаём модель (подробно — в уроке 1.5).
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// Диалог — это срез сообщений с ролями. Здесь система задаёт поведение,
	// а дальше идёт уже состоявшийся обмен (user → assistant) и новый вопрос.
	messages := []*schema.Message{
		schema.SystemMessage("Ты дружелюбный помощник по Go. Отвечай коротко, одним предложением."),
		schema.UserMessage("Что выведет fmt.Println(1 + 1)?"),
		schema.AssistantMessage("Выведет 2.", nil), // прошлый ответ модели — часть истории
		schema.UserMessage(`А что выведет fmt.Println("1" + "1")?`),
	}

	// Покажем, из чего состоит история: у каждого сообщения есть роль и текст.
	fmt.Println("История диалога:")
	for _, m := range messages {
		fmt.Printf("  [%s] %s\n", m.Role, m.Content)
	}

	// Отправляем всю историю модели. Она увидит контекст и ответит на последний вопрос.
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("ошибка генерации: %v", err)
	}

	// Ответ — сообщение с ролью assistant.
	fmt.Printf("\n  [%s] %s\n", response.Role, response.Content)
}
