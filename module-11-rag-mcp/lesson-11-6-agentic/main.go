// module-11-rag-mcp/lesson-11-6-agentic/main.go

// Урок 11.6. От RAG к agentic RAG.
//
// В уроке 11.5 RAG был жёстким графом retrieve -> prompt -> generate. Здесь
// делаем его агентным: поиск становится инструментом react.Agent, и агент сам
// решает, когда искать. А сам инструмент search_kb умнее обычного ретривера:
// переписывает запрос несколькими формулировками и сливает выдачу через RRF
// (см. agentic.go). Ретривер берём тот же, что в 11.4 (store.go).
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделями qwen3.5 (ответ + переписывание запроса) и
// qwen2.5-coder:1.5b (эмбеддинги). Пока процесс работает, граф агента виден в
// EinoDev (devops.Init).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	embollama "github.com/cloudwego/eino-ext/components/embedding/ollama" // эмбеддер
	chatollama "github.com/cloudwego/eino-ext/components/model/ollama"    // чат-модель
	"github.com/cloudwego/eino-ext/devops"                                // сервер EinoDev
	"github.com/cloudwego/eino/components/tool"                           // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"                     // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                                   // ToolsNodeConfig
	"github.com/cloudwego/eino/flow/agent/react"                          // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                                    // Message, Document
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	embedModel    = "qwen2.5-coder:1.5b"     // модель для эмбеддингов
	chatModelName = "qwen3.5"                // модель для ответа и переписывания запроса
	maxStep       = 12                       // лимит шагов ReAct
)

// knowledgeBase — та же маленькая база знаний о курсе, что в уроке 11.5.
var knowledgeBase = []string{
	"Этот курс учит создавать AI-агентов на Go с помощью фреймворка Eino и локальной модели через Ollama.",
	"ReAct — это цикл агента: рассуждение, вызов инструмента, наблюдение результата и так до готового ответа.",
	"Eino собирает агента как граф из узлов: модель, инструменты и ветвления.",
	"RAG добавляет агенту знания: по вектору вопроса находим похожие документы и кладём их в промпт.",
	"MCP — это открытый протокол: внешний сервер отдаёт инструменты, а агент их вызывает.",
	"Mini Code — учебный агент-кодер на Go, который читает и меняет файлы с подтверждением.",
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev — граф агента виден, пока работает процесс.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	embedder, err := embollama.NewEmbedder(ctx, &embollama.EmbeddingConfig{
		BaseURL: ollamaBaseURL,
		Model:   embedModel,
	})
	if err != nil {
		log.Fatalf("не удалось создать эмбеддер: %v", err)
	}

	// Индексируем базу знаний (тем же ретривером memStore из урока 11.4).
	store := newMemStore(embedder)
	docs := make([]*schema.Document, len(knowledgeBase))
	for i, text := range knowledgeBase {
		docs[i] = &schema.Document{Content: text}
	}
	if _, err := store.Store(ctx, docs); err != nil {
		log.Fatalf("не удалось проиндексировать базу знаний: %v", err)
	}

	chatModel, err := chatollama.NewChatModel(ctx, &chatollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    chatModelName,
		Thinking: &chatollama.ThinkValue{Value: false},
	})
	if err != nil {
		log.Fatalf("не удалось создать чат-модель: %v", err)
	}

	// Поиск — это инструмент агента (agentic RAG). Модель нужна для переписывания запроса.
	searchTool, err := utils.InferTool("search_kb",
		"Ищет в базе знаний курса по смыслу (переписывает запрос и сливает выдачу RRF)", makeSearchKB(store, chatModel))
	if err != nil {
		log.Fatalf("инструмент search_kb: %v", err)
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{searchTool}},
		MaxStep:          maxStep,
	})
	if err != nil {
		log.Fatalf("не удалось собрать агента: %v", err)
	}

	question := "Что такое RAG и зачем он агенту?"
	msgs := []*schema.Message{
		schema.SystemMessage("Ты помощник по курсу. Если вопрос про курс — сначала найди ответ через search_kb, потом отвечай ТОЛЬКО по найденному. Отвечай на русском, кратко."),
		schema.UserMessage(question),
	}
	answer, err := agent.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("ошибка агента: %v", err)
	}

	fmt.Println("Вопрос:", question)
	fmt.Println("Ответ: ", answer.Content)

	// Держим процесс, чтобы успеть посмотреть граф агента в EinoDev. Ctrl+C — выход.
	log.Println("Готово. Откройте EinoDev (адрес в строке start debug http server). Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
