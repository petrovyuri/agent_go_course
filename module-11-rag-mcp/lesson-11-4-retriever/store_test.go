// module-11-rag-mcp/lesson-11-4-retriever/store_test.go

package main

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// fakeEmbedder — эмбеддер-заглушка: отдаёт заранее заданный вектор для каждого
// известного текста. Так тест поиска не зависит от Ollama.
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
		"щенки":  {0, 0.9, 0.1}, // запрос: ближе всего к "собаки"
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
	if len(found) == 0 {
		t.Fatal("поиск ничего не вернул")
	}
	if found[0].Content != "собаки" {
		t.Errorf("ждали первым 'собаки', получили '%s'", found[0].Content)
	}
	if found[0].Score() <= found[len(found)-1].Score() {
		t.Error("результаты не отсортированы по убыванию близости")
	}
}
