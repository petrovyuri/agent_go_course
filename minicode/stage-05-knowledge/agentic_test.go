// minicode/stage-05-knowledge/agentic_test.go

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeChatModel — модель-заглушка: возвращает заданный ответ. Stream не нужен
// для тестов agentic-поиска.
type fakeChatModel struct{ reply string }

func (f *fakeChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(f.reply, nil), nil
}

func (f *fakeChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil //nolint:nilnil // заглушка: Stream в тестах не вызывается
}

func TestExpandQuery(t *testing.T) {
	m := &fakeChatModel{reply: "1. чтение файла\n- как прочитать\nоткрыть файл"}
	got := expandQuery(context.Background(), m, "прочитать файл")
	if got[0] != "прочитать файл" {
		t.Fatalf("первым должен быть исходный запрос: %v", got)
	}
	if len(got) < 2 {
		t.Fatalf("ждали варианты запроса, получили %v", got)
	}
	for _, q := range got {
		if strings.HasPrefix(q, "-") || strings.HasPrefix(q, "1.") {
			t.Errorf("маркер/нумерация не срезаны: %q", q)
		}
	}
}

func TestExpandQueryNilModel(t *testing.T) {
	got := expandQuery(context.Background(), nil, "x")
	if len(got) != 1 || got[0] != "x" {
		t.Errorf("без модели — только исходный запрос, получили %v", got)
	}
}

func TestMultiSearchRRF(t *testing.T) {
	emb := &fakeEmbedder{byText: map[string][]float64{
		"search_query: как читать": {1, 0},
	}}
	idx := &projectIndex{embedder: emb, chunks: []chunk{
		{path: "r.go", line: 1, text: "чтение", vec: []float64{1, 0}},
		{path: "w.go", line: 1, text: "запись", vec: []float64{0, 1}},
	}}
	// reply="" → вариантов нет, ищем только по исходному запросу.
	got, err := multiSearch(context.Background(), idx, &fakeChatModel{reply: ""}, "как читать", 2)
	if err != nil {
		t.Fatalf("multiSearch: %v", err)
	}
	if len(got) == 0 || got[0].text != "чтение" {
		t.Errorf("ждали 'чтение' первым, получили %v", got)
	}
}
