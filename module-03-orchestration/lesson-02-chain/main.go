// Урок 3.2. Chain: линейная последовательность компонентов.
// Собираем цепочку "шаблон промпта → модель" в один исполняемый компонент.
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
	"github.com/cloudwego/eino/compose"                     // оркестрация: Chain
	"github.com/cloudwego/eino/schema"                      // Message и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

func main() {
	ctx := context.Background()

	// Шаблон промпта с переменными (из урока 2.5).
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("Ты помощник по {topic}. Отвечай одним предложением."),
		schema.UserMessage("{question}"),
	)

	// Модель (из модуля 1).
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// Собираем цепочку: на входе — переменные шаблона (map), на выходе — сообщение модели.
	// Шаблон превращает map в []*schema.Message, а его выход уходит на вход модели.
	chain, err := compose.NewChain[map[string]any, *schema.Message]().
		AppendChatTemplate(template).
		AppendChatModel(chatModel).
		Compile(ctx) // Compile проверяет совместимость узлов и собирает исполняемую цепочку
	if err != nil {
		log.Fatalf("не удалось собрать цепочку: %v", err)
	}

	// Запускаем всю цепочку одним вызовом. Передаём только переменные шаблона —
	// остальное (формирование промпта, вызов модели) цепочка делает сама.
	out, err := chain.Invoke(ctx, map[string]any{
		"topic":    "язык Go",
		"question": "Что такое горутина?",
	})
	if err != nil {
		log.Fatalf("ошибка выполнения цепочки: %v", err)
	}

	fmt.Println(out.Content)
}
