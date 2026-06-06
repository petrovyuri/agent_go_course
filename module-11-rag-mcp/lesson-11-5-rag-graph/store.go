// module-11-rag-mcp/lesson-11-5-rag-graph/store.go

package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// topK — сколько самых похожих документов возвращает поиск.
const topK = 3

// entry — один документ вместе с его вектором.
type entry struct {
	doc    *schema.Document
	vector []float64
}

// memStore — хранилище векторов в памяти. Реализует indexer.Indexer и
// retriever.Retriever, поэтому его можно поставить узлом ретривера в граф.
type memStore struct {
	embedder embedding.Embedder
	entries  []entry
}

func newMemStore(e embedding.Embedder) *memStore {
	return &memStore{embedder: e}
}

// Store эмбеддит документы и сохраняет их в память (реализация indexer.Indexer).
func (m *memStore) Store(ctx context.Context, docs []*schema.Document, _ ...indexer.Option) ([]string, error) {
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}
	vectors, err := m.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("эмбеддинг документов: %w", err)
	}
	ids := make([]string, len(docs))
	for i, d := range docs {
		if d.ID == "" {
			d.ID = strconv.Itoa(len(m.entries) + 1)
		}
		m.entries = append(m.entries, entry{doc: d, vector: vectors[i]})
		ids[i] = d.ID
	}
	return ids, nil
}

// Retrieve находит документы, ближайшие к запросу (реализация retriever.Retriever).
func (m *memStore) Retrieve(ctx context.Context, query string, _ ...retriever.Option) ([]*schema.Document, error) {
	qv, err := m.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("эмбеддинг запроса: %w", err)
	}

	type scored struct {
		doc   *schema.Document
		score float64
	}
	ranked := make([]scored, len(m.entries))
	for i, e := range m.entries {
		ranked[i] = scored{doc: e.doc, score: cosine(qv[0], e.vector)}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	n := min(topK, len(ranked))
	out := make([]*schema.Document, n)
	for i := range out {
		out[i] = ranked[i].doc.WithScore(ranked[i].score)
	}
	return out, nil
}

// cosine возвращает косинусную близость двух векторов (1.0 — похожи, 0 — нет).
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
