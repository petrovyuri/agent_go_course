// Урок 3.9. Практика: граф-маршрутизатор запросов.
// Граф классифицирует вопрос и направляет его в подходящий "промпт-узел",
// после чего общий узел-модель отвечает. Итог модуля: граф + ветвление +
// Lambda + модель в одном месте.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст
	"fmt"       // вывод в консоль
	"log"       // log.Fatalf — остановиться с понятной ошибкой
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"strings"   // поиск ключевых слов
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // debug-сервер для плагина EinoDev
	"github.com/cloudwego/eino/compose"                     // оркестрация: Graph, Branch, Lambda
	"github.com/cloudwego/eino/schema"                      // Message и конструкторы
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
)

// isAboutGo грубо определяет, относится ли вопрос к Go.
func isAboutGo(q string) bool {
	q = strings.ToLower(q)
	for _, kw := range []string{"go", "горутин", "канал", "интерфейс", "слайс", "срез"} {
		if strings.Contains(q, kw) {
			return true
		}
	}
	return false
}

func main() {
	ctx := context.Background()

	// Поднимаем debug-сервер EinoDev ДО Compile — собранный граф попадёт в плагин.
	if err := devops.Init(ctx); err != nil {
		log.Fatalf("не удалось запустить EinoDev: %v", err)
	}

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: ollamaBaseURL,
		Model:   modelName,
	})
	if err != nil {
		log.Fatalf("не удалось создать ChatModel: %v", err)
	}

	// Вход графа — вопрос (строка), выход — ответ модели (*schema.Message).
	g := compose.NewGraph[string, *schema.Message]()

	// Узел-вход: пропускает вопрос дальше (его читает ветвление).
	_ = g.AddLambdaNode("intake", compose.InvokableLambda(func(ctx context.Context, q string) (string, error) {
		return q, nil
	}))

	// Два узла-"промпта": каждый строит свой системный промпт под категорию.
	_ = g.AddLambdaNode("goPrompt", compose.InvokableLambda(func(ctx context.Context, q string) ([]*schema.Message, error) {
		return []*schema.Message{
			schema.SystemMessage("Ты опытный Go-разработчик. Отвечай точно и с примерами из Go."),
			schema.UserMessage(q),
		}, nil
	}))
	_ = g.AddLambdaNode("generalPrompt", compose.InvokableLambda(func(ctx context.Context, q string) ([]*schema.Message, error) {
		return []*schema.Message{
			schema.SystemMessage("Ты дружелюбный универсальный помощник. Отвечай кратко."),
			schema.UserMessage(q),
		}, nil
	}))

	// Общий узел-модель: принимает []*schema.Message от любого промпт-узла.
	_ = g.AddChatModelNode("model", chatModel)

	_ = g.AddEdge(compose.START, "intake")

	// Ветвление: по содержанию вопроса выбираем нужный промпт-узел.
	branch := compose.NewGraphBranch(
		func(ctx context.Context, q string) (string, error) {
			if isAboutGo(q) {
				return "goPrompt", nil
			}
			return "generalPrompt", nil
		},
		map[string]bool{"goPrompt": true, "generalPrompt": true},
	)
	_ = g.AddBranch("intake", branch)

	// Обе ветки сходятся в одном узле-модели, а та — в выход графа.
	_ = g.AddEdge("goPrompt", "model")
	_ = g.AddEdge("generalPrompt", "model")
	_ = g.AddEdge("model", compose.END)

	runner, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать граф: %v", err)
	}

	// Два вопроса из разных категорий — увидим, что сработали разные ветки.
	for _, q := range []string{
		"Что такое горутина в Go?",
		"Посоветуй книгу о космосе.",
	} {
		out, err := runner.Invoke(ctx, q)
		if err != nil {
			log.Fatalf("ошибка выполнения графа: %v", err)
		}
		fmt.Printf("Вопрос: %s\nОтвет: %s\n\n", q, out.Content)
	}

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф.
	// Откройте панель Eino Dev в IDE и запускайте узлы с тестовым входом. Ctrl+C — выход.
	log.Println("EinoDev запущен. Откройте плагин Eino Dev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
