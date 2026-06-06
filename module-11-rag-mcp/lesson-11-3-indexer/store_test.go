// module-11-rag-mcp/lesson-11-3-indexer/store_test.go

package main

import (
	"context"
	"math"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// fakeEmbedder — эмбеддер-заглушка для тестов: выдаёт фиксированные векторы и не
// ходит в сеть. Так тест не зависит от запущенного Ollama.
type fakeEmbedder struct {
	vectors [][]float64
}

func (f *fakeEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	return f.vectors[:len(texts)], nil
}

func TestCosine(t *testing.T) {
	if got := cosine([]float64{1, 0}, []float64{1, 0}); math.Abs(got-1) > 1e-9 {
		t.Errorf("одинаковые векторы: ждали 1.0, получили %v", got)
	}
	if got := cosine([]float64{1, 0}, []float64{0, 1}); math.Abs(got) > 1e-9 {
		t.Errorf("перпендикулярные векторы: ждали 0, получили %v", got)
	}
	if got := cosine([]float64{0, 0}, []float64{1, 1}); got != 0 {
		t.Errorf("нулевой вектор: ждали 0, получили %v", got)
	}
}

func TestStore(t *testing.T) {
	emb := &fakeEmbedder{vectors: [][]float64{{1, 0}, {0, 1}}}
	store := newMemStore(emb)

	docs := []*schema.Document{
		{Content: "первый"},
		{Content: "второй"},
	}
	ids, err := store.Store(context.Background(), docs)
	if err != nil {
		t.Fatalf("Store вернул ошибку: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ждали 2 ID, получили %d", len(ids))
	}
	if len(store.entries) != 2 {
		t.Fatalf("ждали 2 записи в хранилище, получили %d", len(store.entries))
	}
	if store.entries[0].doc.ID == "" {
		t.Error("документу не присвоен ID")
	}
}
