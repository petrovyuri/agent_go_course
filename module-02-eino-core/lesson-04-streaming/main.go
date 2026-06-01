// Урок 2.4. Стриминг: StreamReader и обработка чанков.
// Читаем ответ по кусочкам: печатаем вживую и параллельно собираем полный текст.
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
	"io"      // io.EOF — признак конца потока
	"log"     // log.Fatalf — остановиться с понятной ошибкой
	"strings" // strings.Builder — копим полный ответ из чанков

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

	messages := []*schema.Message{
		schema.SystemMessage("Ты помощник по Go. Объясняй просто, на несколько предложений."),
		schema.UserMessage("Что такое каналы (channels) в Go и зачем они нужны?"),
	}

	// Открываем поток. С этого момента модель начинает генерировать ответ,
	// а мы будем забирать его по кусочкам, не дожидаясь конца.
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("ошибка Stream: %v", err)
	}
	defer stream.Close() // поток держит ресурсы (соединение) — закрыть обязательно

	var full strings.Builder // сюда соберём полный ответ из чанков
	chunks := 0              // посчитаем, на сколько кусочков разбился ответ

	for {
		chunk, err := stream.Recv() // следующий кусочек ответа
		if errors.Is(err, io.EOF) {
			break // io.EOF — это не ошибка, а нормальный конец потока
		}
		if err != nil {
			log.Fatalf("ошибка чтения потока: %v", err)
		}

		fmt.Print(chunk.Content)        // показываем кусочек сразу (эффект живого набора)
		full.WriteString(chunk.Content) // и копим его в полный текст
		chunks++
	}

	// Когда поток закончился, у нас на руках весь ответ целиком.
	answer := full.String()
	fmt.Printf("\n\n--- собрано чанков: %d, символов: %d ---\n", chunks, len([]rune(answer)))
}
