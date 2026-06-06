// module-11-rag-mcp/lesson-11-6-agentic/agentic_test.go

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeEmbedder — эмбеддер-заглушка: отдаёт заранее заданный вектор для текста.
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

// fakeChat — чат-модель-заглушка: на Generate отдаёт фиксированный текст. Так
// тест переписывания запроса не зависит от Ollama. Stream здесь не используется.
type fakeChat struct {
	reply string
}

func (f *fakeChat) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(f.reply, nil), nil
}

func (f *fakeChat) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream не используется в тестах")
}

func TestExpandQuery(t *testing.T) {
	// Без модели — только исходный запрос.
	if got := expandQuery(context.Background(), nil, "запрос"); len(got) != 1 || got[0] != "запрос" {
		t.Errorf("nil-модель: ждали [запрос], получили %v", got)
	}
	// С моделью — исходный плюс разобранные строки (нумерация/маркеры срезаются).
	m := &fakeChat{reply: "1. как читать файл\n- чтение данных"}
	got := expandQuery(context.Background(), m, "прочитать файл")
	if len(got) != 3 || got[0] != "прочитать файл" || got[1] != "как читать файл" || got[2] != "чтение данных" {
		t.Errorf("разбор вариантов неверный: %v", got)
	}
}

func TestSearchKBRanksClosest(t *testing.T) {
	emb := &fakeEmbedder{byText: map[string][]float64{
		"чтение файла":   {1, 0, 0},
		"запись файла":   {0, 1, 0},
		"запуск команды": {0, 0, 1},
		"как читать":     {0.9, 0.1, 0}, // ближе всего к "чтение файла"
	}}
	store := newMemStore(emb)
	docs := []*schema.Document{
		{Content: "чтение файла"}, {Content: "запись файла"}, {Content: "запуск команды"},
	}
	if _, err := store.Store(context.Background(), docs); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// m=nil → один запрос, RRF над одним списком; проверяем формат и порядок.
	out, err := makeSearchKB(store, nil)(context.Background(), searchKBArgs{Query: "как читать"})
	if err != nil {
		t.Fatalf("search_kb: %v", err)
	}
	if !strings.HasPrefix(out, "- чтение файла") {
		t.Errorf("первым должен быть 'чтение файла', получили:\n%s", out)
	}
}
