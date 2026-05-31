// Программа проверяет, что окружение готово к курсу: Ollama запущена и видит модели.
// Намеренно без Eino — только стандартная библиотека Go. Так мы убеждаемся, что Go
// достучится до Ollama по тому же адресу, по которому в него будет ходить наш агент.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// ollamaBaseURL — адрес локального сервера Ollama по умолчанию.
const ollamaBaseURL = "http://localhost:11434"

// tagsResponse повторяет форму ответа Ollama на GET /api/tags.
// Нам нужно только имя модели, остальные поля опускаем.
type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := fetchModels(ctx, ollamaBaseURL)
	if err != nil {
		fmt.Printf("✗ Не удалось связаться с Ollama на %s\n", ollamaBaseURL)
		fmt.Printf("  Причина: %v\n", err)
		fmt.Println("  Запустите сервер командой `ollama serve` и попробуйте снова.")
		os.Exit(1)
	}

	fmt.Printf("✓ Ollama отвечает на %s\n", ollamaBaseURL)
	fmt.Printf("✓ Найдено моделей: %d\n", len(models))
	for _, name := range models {
		fmt.Printf("  - %s\n", name)
	}

	if len(models) == 0 {
		fmt.Println("\nМоделей нет. Скачайте модель: `ollama pull qwen3.5`")
		os.Exit(1)
	}

	fmt.Println("\nОкружение готово. Можно идти в урок 1.5.")
}

// fetchModels запрашивает у Ollama список установленных моделей.
// Адрес передаётся параметром — так функцию удобно протестировать через httptest
// без запущенной Ollama.
func fetchModels(ctx context.Context, baseURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
	}

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("не разобрать ответ: %w", err)
	}

	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	return names, nil
}
