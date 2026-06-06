// module-11-rag-mcp/lesson-11-3-indexer/main.go

// Урок 11.3. Indexer и хранилище векторов.
//
// Indexer — это компонент, который складывает документы в векторное хранилище.
// Здесь мы пишем своё хранилище в памяти (memStore, см. store.go), которое
// реализует интерфейс indexer.Indexer из Eino. На входе — документы, внутри они
// превращаются в векторы и сохраняются.
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

// knowledgeBase — наша маленькая база знаний о курсе. В реальном проекте сюда
// попадают куски документации, статей или кода.
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

	// Превращаем строки в документы и индексируем их.
	docs := make([]*schema.Document, len(knowledgeBase))
	for i, text := range knowledgeBase {
		docs[i] = &schema.Document{Content: text}
	}

	ids, err := store.Store(ctx, docs)
	if err != nil {
		log.Fatalf("не удалось проиндексировать документы: %v", err)
	}

	fmt.Printf("Проиндексировано документов: %d (ID: %v)\n", len(ids), ids)
	fmt.Printf("В хранилище записей: %d, размерность вектора: %d\n",
		len(store.entries), len(store.entries[0].vector))
}
