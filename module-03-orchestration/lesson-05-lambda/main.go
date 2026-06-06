// Урок 3.5. Lambda-узлы: вставка произвольной Go-логики в граф.
// После модели ставим свой узел-функцию, который обрабатывает ответ.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст: таймаут и отмена запроса
	"fmt"       // вывод в консоль
	"log"       // log.Fatalf — остановиться с понятной ошибкой
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"strings"   // обработка текста в Lambda
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // debug-сервер для плагина EinoDev
	"github.com/cloudwego/eino/components/prompt"           // шаблоны промптов
	"github.com/cloudwego/eino/compose"                     // оркестрация: Graph, Lambda
	"github.com/cloudwego/eino/schema"                      // Message и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

func main() {
	ctx := context.Background()

	// Поднимаем debug-сервер EinoDev ДО Compile — собранный граф попадёт в плагин.
	if err := devops.Init(ctx); err != nil {
		log.Fatalf("не удалось запустить EinoDev: %v", err)
	}

	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("Ты помощник по {topic}. Отвечай одним предложением."),
		schema.UserMessage("{question}"),
	)

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// Граф: вход — переменные шаблона (map), выход — строка (обработанный ответ).
	g := compose.NewGraph[map[string]any, string]()

	_ = g.AddChatTemplateNode("template", template)
	_ = g.AddChatModelNode("model", chatModel)

	// Lambda-узел — это обычная Go-функция (ctx, вход) -> (выход, error),
	// обёрнутая в compose.InvokableLambda. Здесь он берёт ответ модели
	// (*schema.Message) и превращает его в красиво оформленную строку.
	format := compose.InvokableLambda(func(ctx context.Context, m *schema.Message) (string, error) {
		text := strings.TrimSpace(m.Content)
		return "💡 " + text, nil
	})
	_ = g.AddLambdaNode("format", format)

	// START → template → model → format → END.
	_ = g.AddEdge(compose.START, "template")
	_ = g.AddEdge("template", "model")
	_ = g.AddEdge("model", "format")
	_ = g.AddEdge("format", compose.END)

	runner, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать граф: %v", err)
	}

	out, err := runner.Invoke(ctx, map[string]any{
		"topic":    "язык Go",
		"question": "Что такое интерфейс?",
	})
	if err != nil {
		log.Fatalf("ошибка выполнения графа: %v", err)
	}

	// out — это уже строка, которую вернул наш Lambda-узел.
	fmt.Println(out)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф.
	// Откройте панель Eino Dev в IDE и запускайте узлы с тестовым входом. Ctrl+C — выход.
	log.Println("EinoDev запущен. Откройте плагин Eino Dev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
