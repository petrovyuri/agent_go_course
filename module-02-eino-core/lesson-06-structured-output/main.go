// Урок 2.6. Структурированный вывод: парсинг JSON-ответов модели.
// Просим модель ответить строго в JSON и разбираем ответ в структуру Go.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"       // контекст: таймаут и отмена запроса
	"encoding/json" // разбор JSON в структуру Go
	"fmt"           // вывод в консоль
	"log"           // log.Fatalf — остановиться с понятной ошибкой
	"strings"       // вырезаем JSON из ответа модели

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
	"github.com/cloudwego/eino/components/model"            // опции вызова: WithTemperature
	"github.com/cloudwego/eino/schema"                      // Message и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

// Recipe — структура, в которую разберём ответ модели. Теги json задают имена полей.
type Recipe struct {
	Name        string   `json:"name"`        // название блюда
	Minutes     int      `json:"minutes"`     // время приготовления
	Ingredients []string `json:"ingredients"` // список ингредиентов
}

func main() {
	ctx := context.Background()

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// В системном промпте описываем нужный формат — это наш "контракт" с моделью.
	messages := []*schema.Message{
		schema.SystemMessage(`Ты возвращаешь ТОЛЬКО валидный JSON, без markdown и пояснений.
Схема: {"name": строка, "minutes": число, "ingredients": массив строк}.`),
		schema.UserMessage("Дай простой рецепт омлета."),
	}

	// Низкая температура — для структурированного вывода нужна стабильность, не креатив.
	response, err := chatModel.Generate(ctx, messages, model.WithTemperature(0.0))
	if err != nil {
		log.Fatalf("ошибка генерации: %v", err)
	}

	// Модель иногда добавляет ```json или текст вокруг. Берём кусок от первой { до последней }.
	raw := extractJSON(response.Content)

	// Разбираем JSON в структуру Go.
	var recipe Recipe
	if err := json.Unmarshal([]byte(raw), &recipe); err != nil {
		log.Fatalf("не удалось разобрать JSON: %v\nответ модели был: %s", err, response.Content)
	}

	// Теперь это обычные данные Go — с ними можно работать в коде.
	fmt.Printf("Блюдо: %s\n", recipe.Name)
	fmt.Printf("Время: %d мин\n", recipe.Minutes)
	fmt.Println("Ингредиенты:")
	for _, ing := range recipe.Ingredients {
		fmt.Printf("  - %s\n", ing)
	}
}

// extractJSON вырезает JSON-объект из ответа модели: от первой { до последней }.
// Так мы устойчивы к обёрткам вроде ```json ... ``` и лишнему тексту вокруг.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
