// Урок 3.3. Визуальная отладка графа: плагин EinoDev.
// Поднимаем debug-сервер EinoDev, собираем знакомую цепочку из урока 3.2
// (шаблон + модель) и держим процесс живым, чтобы плагин в IDE мог
// подключиться, показать её схему и запускать узлы с тестовым входом.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Затем в IDE откройте плагин Eino Dev и подключитесь к локальному серверу.
package main

import (
	"context"   // контекст
	"log"       // логирование
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/components/model/ollama" // реализация ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // debug-сервер для плагина EinoDev
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

	// 1. Поднимаем debug-сервер EinoDev. Делать это нужно ДО Compile.
	if err := devops.Init(ctx); err != nil {
		log.Fatalf("не удалось запустить EinoDev: %v", err)
	}

	// 2. Собираем ту же цепочку, что и в уроке 3.2: шаблон + модель.
	//    После Compile она автоматически появится в плагине.
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

	_, err = compose.NewChain[map[string]any, *schema.Message]().
		AppendChatTemplate(template).
		AppendChatModel(chatModel).
		Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать цепочку: %v", err)
	}

	log.Println("EinoDev запущен. Откройте плагин Eino Dev в IDE и подключитесь.")
	log.Println("Для выхода нажмите Ctrl+C.")

	// 3. Держим процесс живым: пока программа работает, плагин может
	//    показывать цепочку и запускать узлы. Выходим по Ctrl+C.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	log.Println("Завершение работы.")
}
