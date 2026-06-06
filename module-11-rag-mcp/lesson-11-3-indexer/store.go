// module-11-rag-mcp/lesson-11-3-indexer/store.go

package main

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

// entry — один документ вместе с его вектором.
type entry struct {
	doc    *schema.Document
	vector []float64
}

// memStore — простое хранилище векторов в оперативной памяти. Оно реализует
// интерфейс indexer.Indexer из Eino, поэтому его можно поставить узлом в граф.
// В бою вместо него берут Redis, Milvus или pgvector — интерфейс тот же.
type memStore struct {
	embedder embedding.Embedder
	entries  []entry
}

// newMemStore создаёт пустое хранилище, которое будет считать векторы эмбеддером.
func newMemStore(e embedding.Embedder) *memStore {
	return &memStore{embedder: e}
}

// Store эмбеддит содержимое документов и складывает их в память. Это и есть
// реализация indexer.Indexer: на вход документы, на выходе их ID.
func (m *memStore) Store(ctx context.Context, docs []*schema.Document, _ ...indexer.Option) ([]string, error) {
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}

	// Один вызов эмбеддера на все документы сразу — так быстрее.
	vectors, err := m.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("эмбеддинг документов: %w", err)
	}

	ids := make([]string, len(docs))
	for i, d := range docs {
		if d.ID == "" {
			d.ID = strconv.Itoa(len(m.entries) + 1) // простой автоинкремент
		}
		m.entries = append(m.entries, entry{doc: d, vector: vectors[i]})
		ids[i] = d.ID
	}
	return ids, nil
}

// cosine возвращает косинусную близость двух векторов: ближе к 1.0 — тексты
// похожи по смыслу, ближе к 0 — не связаны.
func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
