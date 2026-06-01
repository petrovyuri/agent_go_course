// Урок 2.2. Интерфейс ChatModel: Generate и Stream.
// Один и тот же вопрос задаём двумя способами: Generate ждёт ответ целиком,
// Stream отдаёт его по кусочкам (как печатающаяся строка).
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст: таймаут и отмена запроса
	"errors"  // errors.Is для проверки конца потока
	"fmt"     // вывод в консоль
	"io"      // io.EOF — признак, что поток закончился
	"log"     // log.Fatalf — остановиться с понятной ошибкой

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
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

	// Один и тот же запрос используем для обоих способов.
	messages := []*schema.Message{
		schema.SystemMessage("Ты помощник по Go. Отвечай в двух-трёх предложениях."),
		schema.UserMessage("Что такое горутина и чем она отличается от обычного потока ОС?"),
	}

	// Способ 1. Generate — отправляем запрос и ждём ВЕСЬ ответ целиком.
	fmt.Println("=== Generate (весь ответ сразу) ===")
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("ошибка Generate: %v", err)
	}
	fmt.Println(response.Content)

	// Способ 2. Stream — получаем ответ по кусочкам (чанкам), как живой набор текста.
	fmt.Println("\n=== Stream (по кусочкам) ===")
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("ошибка Stream: %v", err)
	}
	defer stream.Close() // поток нужно обязательно закрыть

	for {
		chunk, err := stream.Recv() // читаем очередной кусочек ответа
		if errors.Is(err, io.EOF) {
			break // поток закончился — ответ получен полностью
		}
		if err != nil {
			fmt.Printf("\nошибка чтения потока: %v\n", err)
			return // defer stream.Close() отработает при выходе из функции
		}
		fmt.Print(chunk.Content) // печатаем кусочек сразу, не дожидаясь остального
	}
	fmt.Println()
}
