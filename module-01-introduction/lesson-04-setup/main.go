// Программа проверяет, что окружение готово к курсу: Ollama запущена и видит модели.
// Намеренно без Eino — только стандартная библиотека Go. Так мы убеждаемся, что Go
// достучится до Ollama по тому же адресу, по которому в него будет ходить наш агент.
package main

import (
	"context"       // контекст: таймаут и отмена запроса
	"encoding/json" // разбор JSON-ответа Ollama
	"errors"        // errors.New для сигнальной ошибки
	"fmt"           // вывод в консоль и сборка ошибок
	"net/http"      // HTTP-клиент для обращения к Ollama
	"os"            // os.Exit для кода возврата при ошибке
	"time"          // длительность таймаута
)

// ollamaBaseURL — адрес локального сервера Ollama по умолчанию.
// Именно сюда Ollama принимает HTTP-запросы после команды `ollama serve`.
const ollamaBaseURL = "http://localhost:11434"

// tagsResponse повторяет форму ответа Ollama на GET /api/tags.
// Описываем только нужное поле — имя модели; остальные поля JSON Go пропустит.
type tagsResponse struct {
	Models []struct {
		Name string `json:"name"` // имя модели, например "qwen3.5"
	} `json:"models"`
}

func main() {
	// main держим тонким: вся логика в run, а os.Exit вызываем только здесь —
	// уже после того как отработают все defer внутри run.
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run проверяет окружение и возвращает ошибку вместо вызова os.Exit.
// Так отложенный cancel() гарантированно отработает при любом выходе.
func run() error {
	// Ограничиваем всё обращение пятью секундами: если сервер завис, не зависнем сами.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // освобождаем ресурсы контекста при выходе

	// Запрашиваем у Ollama список установленных моделей.
	models, err := fetchModels(ctx, ollamaBaseURL)
	if err != nil {
		// Сюда попадаем, если сервер недоступен — печатаем понятную подсказку.
		fmt.Printf("✗ Не удалось связаться с Ollama на %s\n", ollamaBaseURL)
		fmt.Printf("  Причина: %v\n", err)
		fmt.Println("  Запустите сервер командой `ollama serve` и попробуйте снова.")
		return fmt.Errorf("ollama недоступна: %w", err)
	}

	// Всё хорошо: показываем адрес и найденные модели.
	fmt.Printf("✓ Ollama отвечает на %s\n", ollamaBaseURL)
	fmt.Printf("✓ Найдено моделей: %d\n", len(models))
	for _, name := range models {
		fmt.Printf("  - %s\n", name)
	}

	// Список пуст — значит модель ещё не скачана; подсказываем команду.
	if len(models) == 0 {
		fmt.Println("\nМоделей нет. Скачайте модель: `ollama pull qwen3.5`")
		return errors.New("модели не найдены")
	}

	fmt.Println("\nОкружение готово. Можно идти в урок 1.5.")
	return nil
}

// fetchModels запрашивает у Ollama список установленных моделей.
// Адрес передаётся параметром — так функцию удобно протестировать через httptest
// без запущенной Ollama.
func fetchModels(ctx context.Context, baseURL string) ([]string, error) {
	// Готовим GET-запрос к /api/tags с нашим контекстом (таймаут и отмена).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err // например, некорректный URL
	}

	// Отправляем запрос. Ошибка здесь обычно означает, что сервер не запущен.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // тело ответа нужно обязательно закрыть

	// Ollama при успехе отвечает 200; любой другой статус считаем ошибкой.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
	}

	// Разбираем JSON прямо из тела ответа в нашу структуру.
	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("не разобрать ответ: %w", err)
	}

	// Собираем только имена моделей в плоский срез строк.
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	return names, nil
}
