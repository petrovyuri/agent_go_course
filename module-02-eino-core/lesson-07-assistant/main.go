// Урок 2.7. Практика: мини-ассистент с системным промптом и историей.
// Ведём диалог в цикле: читаем ввод, копим историю, отвечаем с учётом контекста.
// Это итог модуля 2 — здесь сходятся сообщения, роли, история и Generate.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"bufio"   // построчное чтение ввода пользователя
	"context" // контекст: таймаут и отмена запроса
	"fmt"     // вывод в консоль
	"log"     // log.Printf — сообщить об ошибке, не прерывая диалог
	"os"      // os.Stdin — стандартный ввод
	"strings" // обрезка пробелов во вводе

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

	// История начинается с системного сообщения — оно задаёт характер ассистента.
	// Дальше история будет расти: к ней добавляются реплики пользователя и ответы модели.
	messages := []*schema.Message{
		schema.SystemMessage("Ты дружелюбный помощник по языку Go. Отвечай кратко и по делу."),
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Мини-ассистент по Go. Задайте вопрос или напишите \"выход\".")

	for {
		fmt.Print("\nВы: ")
		if !scanner.Scan() {
			break // ввод закончился (Ctrl+D / Ctrl+Z)
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue // пустую строку пропускаем
		}
		if text == "выход" || text == "exit" {
			fmt.Println("Пока!")
			break
		}

		// Добавляем вопрос пользователя в историю.
		messages = append(messages, schema.UserMessage(text))

		// Спрашиваем модель — она видит всю историю и отвечает с учётом контекста.
		response, err := chatModel.Generate(ctx, messages)
		if err != nil {
			log.Printf("ошибка генерации: %v", err)
			messages = messages[:len(messages)-1] // откатываем неудачный вопрос
			continue
		}

		fmt.Println("Бот:", response.Content)

		// Ответ модели — это *schema.Message с ролью assistant. Кладём его в историю,
		// чтобы в следующей реплике модель помнила, что уже сказала.
		messages = append(messages, response)
	}
}
