// minicode/stage-05-knowledge/index_test.go

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
)

// fakeEmbedder — эмбеддер-заглушка: отдаёт заранее заданный вектор для текста.
// Так тесты индекса не зависят от запущенного Ollama.
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

func TestSplitChunks(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < chunkLines+5; i++ { // чуть больше одного куска
		sb.WriteString("строка\n")
	}
	chunks := splitChunks("a.go", sb.String())
	if len(chunks) != 2 {
		t.Fatalf("ждали 2 куска, получили %d", len(chunks))
	}
	if chunks[0].line != 1 || chunks[1].line != chunkLines+1 {
		t.Errorf("неверные номера строк: %d, %d", chunks[0].line, chunks[1].line)
	}
	if splitChunks("b.go", "\n\n  \n") != nil {
		t.Error("пустой файл не должен давать кусков")
	}
}

func TestSearchRanksClosest(t *testing.T) {
	emb := &fakeEmbedder{byText: map[string][]float64{
		"чтение файла":             {1, 0, 0},
		"запись файла":             {0, 1, 0},
		"запуск команды":           {0, 0, 1},
		"search_query: как читать": {0.9, 0.1, 0}, // запрос (с префиксом) ближе к "чтение файла"
	}}
	idx := &projectIndex{embedder: emb}
	idx.chunks = []chunk{
		{path: "r.go", line: 1, text: "чтение файла", vec: emb.byText["чтение файла"]},
		{path: "w.go", line: 1, text: "запись файла", vec: emb.byText["запись файла"]},
		{path: "c.go", line: 1, text: "запуск команды", vec: emb.byText["запуск команды"]},
	}
	hits, err := idx.search(context.Background(), "как читать", 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("ждали 2 результата, получили %d", len(hits))
	}
	if hits[0].text != "чтение файла" {
		t.Errorf("первым должен быть 'чтение файла', получили %q", hits[0].text)
	}
}
