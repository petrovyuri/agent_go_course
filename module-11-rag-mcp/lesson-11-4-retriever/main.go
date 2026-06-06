// module-11-rag-mcp/lesson-11-4-retriever/main.go

// Урок 11.4. Retriever: поиск похожих документов.
//
// Retriever — компонент, который по тексту запроса возвращает похожие документы.
// Наше хранилище memStore (см. store.go) теперь умеет и складывать (Store), и
// искать (Retrieve). Поиск работает так: эмбеддим запрос, сравниваем его вектор
// с векторами всех документов косинусной близостью и берём top-K самых близких.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделью qwen2.5-coder:1.5b.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/ollama" // эмбеддер Ollama
	"github.com/cloudwego/eino/schema"                          // Document
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	embedModel    = "qwen2.5-coder:1.5b"     // модель для эмбеддингов
)

// knowledgeBase — маленькая база знаний о курсе.
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

	embedder, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		BaseURL: ollamaBaseURL,
		Model:   embedModel,
	})
	if err != nil {
		log.Fatalf("не удалось создать эмбеддер: %v", err)
	}

	store := newMemStore(embedder)

	// Индексируем базу знаний.
	docs := make([]*schema.Document, len(knowledgeBase))
	for i, text := range knowledgeBase {
		docs[i] = &schema.Document{Content: text}
	}
	if _, err := store.Store(ctx, docs); err != nil {
		log.Fatalf("не удалось проиндексировать документы: %v", err)
	}

	// Ищем самые похожие документы под вопрос пользователя.
	query := "Как дать агенту дополнительные знания?"
	found, err := store.Retrieve(ctx, query)
	if err != nil {
		log.Fatalf("не удалось выполнить поиск: %v", err)
	}

	fmt.Printf("Запрос: %s\n\nНайдено (top-%d):\n", query, topK)
	for i, d := range found {
		fmt.Printf("%d. [%.3f] %s\n", i+1, d.Score(), d.Content)
	}
}
