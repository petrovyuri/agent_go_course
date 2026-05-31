// Первый вызов языковой модели через Eino + Ollama.
// Запуск из папки урока:
//
//	go run .
//
// Перед запуском убедитесь, что Ollama запущена и модель скачана (см. урок 1.4):
//
//	ollama pull qwen3.5
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/schema"
)

const (
	ollamaBaseURL = "http://localhost:11434"
	modelName     = "qwen3.5"
)

func main() {
	ctx := context.Background()

	// 1. Создаём ChatModel — обёртку над моделью в Ollama.
	//    ChatModel в Eino — это интерфейс. Здесь мы берём его реализацию для Ollama,
	//    но точно так же могли бы подставить OpenAI, не меняя остальной код.
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// 2. Готовим сообщения. Модель принимает не строку, а список сообщений с ролями.
	//    System задаёт поведение, User — это запрос пользователя.
	messages := []*schema.Message{
		schema.SystemMessage("Ты лаконичный помощник. Отвечай одним предложением."),
		schema.UserMessage("Объясни, что такое горутина в Go."),
	}

	// 3. Generate отправляет сообщения модели и ждёт полный ответ.
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("ошибка генерации: %v", err)
	}

	// 4. Ответ — тоже *schema.Message, с ролью Assistant. Нас интересует его текст.
	fmt.Println("Ответ модели:")
	fmt.Println(response.Content)
}
