// module-11-rag-mcp/lesson-11-5-rag-graph/store_test.go

package main

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// fakeEmbedder — эмбеддер-заглушка: отдаёт заранее заданный вектор для каждого
// известного текста. Так тесты не зависят от Ollama.
type fakeEmbedder struct {
	byText map[string][]float64
}

func (f *fakeEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		out[i] = f.byText[t]
	}
	return out, nil
}

func TestRetrieveRanksClosestFirst(t *testing.T) {
	emb := &fakeEmbedder{byText: map[string][]float64{
		"кошки":  {1, 0, 0},
		"собаки": {0, 1, 0},
		"камни":  {0, 0, 1},
		"щенки":  {0, 0.9, 0.1},
	}}
	store := newMemStore(emb)

	docs := []*schema.Document{{Content: "кошки"}, {Content: "собаки"}, {Content: "камни"}}
	if _, err := store.Store(context.Background(), docs); err != nil {
		t.Fatalf("Store вернул ошибку: %v", err)
	}

	found, err := store.Retrieve(context.Background(), "щенки")
	if err != nil {
		t.Fatalf("Retrieve вернул ошибку: %v", err)
	}
	if len(found) == 0 || found[0].Content != "собаки" {
		t.Fatalf("ждали первым 'собаки', получили %v", found)
	}
}
