// Урок 3.4. Graph: узлы, рёбра, START и END.
// Тот же поток "шаблон → модель", что и в уроке 3.3, но собран как граф.
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
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // debug-сервер для плагина EinoDev
	"github.com/cloudwego/eino/components/prompt"           // шаблоны промптов
	"github.com/cloudwego/eino/compose"                     // оркестрация: Graph, START, END
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

	// Граф: вход — переменные шаблона (map), выход — сообщение модели.
	g := compose.NewGraph[map[string]any, *schema.Message]()

	// Добавляем узлы. У каждого узла есть имя — по нему мы строим рёбра.
	if err := g.AddChatTemplateNode("template", template); err != nil {
		log.Fatalf("узел template: %v", err)
	}
	if err := g.AddChatModelNode("model", chatModel); err != nil {
		log.Fatalf("узел model: %v", err)
	}

	// Рёбра задают порядок: START → template → model → END.
	// START и END — это "вход" и "выход" графа.
	_ = g.AddEdge(compose.START, "template")
	_ = g.AddEdge("template", "model")
	_ = g.AddEdge("model", compose.END)

	// Compile проверяет узлы и рёбра и собирает исполняемый граф.
	runner, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать граф: %v", err)
	}

	out, err := runner.Invoke(ctx, map[string]any{
		"topic":    "язык Go",
		"question": "Что такое канал (channel)?",
	})
	if err != nil {
		log.Fatalf("ошибка выполнения графа: %v", err)
	}

	fmt.Println(out.Content)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф.
	// Откройте панель Eino Dev в IDE и запускайте узлы с тестовым входом. Ctrl+C — выход.
	log.Println("EinoDev запущен. Откройте плагин Eino Dev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
