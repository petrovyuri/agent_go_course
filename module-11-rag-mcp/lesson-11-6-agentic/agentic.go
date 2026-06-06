// module-11-rag-mcp/lesson-11-6-agentic/agentic.go

// Агентный поиск по базе знаний. Инструмент search_kb сначала просит модель
// переписать запрос несколькими формулировками (query rewriting), ищет по каждой
// через ретривер из урока 11.4 и сливает результаты ранговым слиянием RRF. Это
// и есть шаг от обычного RAG (жёсткий граф из 11.5) к agentic RAG: поиском
// управляет агент, а сам поиск стал умнее.
package main

import (
	"context"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/model" // BaseChatModel для переписывания запроса
	"github.com/cloudwego/eino/schema"           // Document
)

const (
	expandN = 3  // сколько вариантов запроса максимум добавляем
	rrfK    = 60 // константа Reciprocal Rank Fusion
)

// searchKBArgs — параметр инструмента search_kb.
type searchKBArgs struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"что искать в базе знаний по смыслу"`
}

// expandQuery просит модель переписать запрос несколькими формулировками
// (синонимы, другие слова). Всегда включает исходный запрос; при ошибке — только его.
func expandQuery(ctx context.Context, m model.BaseChatModel, query string) []string {
	out := []string{query}
	if m == nil {
		return out
	}
	msgs := []*schema.Message{
		schema.SystemMessage("Перепиши запрос 2-3 разными формулировками для семантического поиска (синонимы, другие слова). Выведи только формулировки, по одной на строку, без нумерации."),
		schema.UserMessage(query),
	}
	resp, err := m.Generate(ctx, msgs)
	if err != nil || resp == nil {
		return out
	}
	for line := range strings.SplitSeq(resp.Content, "\n") {
		s := strings.TrimSpace(strings.TrimLeft(line, "-*0123456789.) "))
		if s != "" && s != query {
			out = append(out, s)
		}
		if len(out) >= expandN+1 {
			break
		}
	}
	return out
}

// makeSearchKB возвращает инструмент search_kb: переписываем запрос, ищем по
// каждой формулировке ретривером и сливаем выдачу через RRF (вклад куска тем
// больше, чем выше его ранг). RRF не зависит от абсолютных оценок.
func makeSearchKB(store *memStore, m model.BaseChatModel) func(context.Context, searchKBArgs) (string, error) {
	return func(ctx context.Context, in searchKBArgs) (string, error) {
		type agg struct {
			doc   *schema.Document
			score float64
		}
		byKey := map[string]*agg{}
		var order []string
		for _, q := range expandQuery(ctx, m, in.Query) {
			docs, err := store.Retrieve(ctx, q)
			if err != nil {
				return "", err
			}
			for rank, d := range docs {
				if byKey[d.ID] == nil {
					byKey[d.ID] = &agg{doc: d}
					order = append(order, d.ID)
				}
				byKey[d.ID].score += 1.0 / float64(rrfK+rank)
			}
		}
		if len(order) == 0 {
			return "ничего не найдено", nil
		}
		sort.SliceStable(order, func(i, j int) bool { return byKey[order[i]].score > byKey[order[j]].score })
		var b strings.Builder
		for _, key := range order[:min(topK, len(order))] {
			b.WriteString("- " + byKey[key].doc.Content + "\n")
		}
		return b.String(), nil
	}
}
