// minicode/stage-05-knowledge/agentic.go

// Agentic RAG: делаем поиск не одним проходом, а умным. Инструмент search_code
// сначала просит модель переписать запрос несколькими формулировками (query
// rewriting), ищет по каждой и сливает результаты ранговым слиянием RRF. Так
// нужный код находится, даже если пользователь описал задачу не теми словами,
// что в коде. Сам момент поиска по-прежнему выбирает агент в ReAct-цикле — это и
// есть agentic RAG.
package main

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/model" // BaseChatModel для переписывания запроса
	"github.com/cloudwego/eino/schema"           // Message
)

const (
	expandN = 3  // сколько вариантов запроса максимум добавляем
	fanoutK = 6  // сколько кусков берём из каждого варианта до слияния
	rrfK    = 60 // константа Reciprocal Rank Fusion (чем больше, тем мягче вес ранга)
)

// searchCodeArgs — параметр инструмента search_code.
type searchCodeArgs struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"что искать по смыслу в коде проекта"`
}

// expandQuery просит модель переписать запрос несколькими формулировками
// (синонимы, термины из кода). Всегда включает исходный запрос; при любой ошибке
// возвращает только его.
func expandQuery(ctx context.Context, m model.BaseChatModel, query string) []string {
	out := []string{query}
	if m == nil {
		return out
	}
	msgs := []*schema.Message{
		schema.SystemMessage("Ты помогаешь искать по коду. Перепиши запрос 2-3 разными формулировками для семантического поиска (синонимы, термины из кода). Выведи только формулировки, по одной на строку, без нумерации."),
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

// multiSearch — агентный поиск: расширяем запрос, ищем по каждому варианту и
// сливаем результаты ранговым слиянием (RRF), затем берём top-K. RRF не зависит
// от абсолютных оценок, поэтому корректно объединяет списки от разных запросов.
func multiSearch(ctx context.Context, idx *projectIndex, m model.BaseChatModel, query string, k int) ([]chunk, error) {
	type agg struct {
		c     chunk
		score float64
	}
	byKey := map[string]*agg{}
	var order []string
	for _, q := range expandQuery(ctx, m, query) {
		hits, err := idx.search(ctx, q, fanoutK)
		if err != nil {
			return nil, err
		}
		for rank, h := range hits {
			key := h.path + ":" + strconv.Itoa(h.line)
			if byKey[key] == nil {
				byKey[key] = &agg{c: h}
				order = append(order, key)
			}
			byKey[key].score += 1.0 / float64(rrfK+rank) // вклад тем больше, чем выше ранг
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return byKey[order[i]].score > byKey[order[j]].score })
	out := make([]chunk, 0, k)
	for _, key := range order {
		if len(out) >= k {
			break
		}
		out = append(out, byKey[key].c)
	}
	return out, nil
}

// makeSearchCode — инструмент search_code: агентный семантический поиск
// (расширение запроса моделью + RRF). Это RAG внутри агента: retrieve, которым
// управляет сам агент, вызывая инструмент когда нужно.
func makeSearchCode(idx *projectIndex, m model.BaseChatModel) func(context.Context, searchCodeArgs) (string, error) {
	return func(ctx context.Context, in searchCodeArgs) (string, error) {
		hits, err := multiSearch(ctx, idx, m, in.Query, indexTopK)
		if err != nil {
			return "", err
		}
		if len(hits) == 0 {
			return "индекс пуст или ничего не найдено", nil
		}
		var b strings.Builder
		for _, h := range hits {
			b.WriteString(h.path + ":" + strconv.Itoa(h.line) + "\n")
			b.WriteString(h.text)
			b.WriteString("\n---\n")
		}
		return b.String(), nil
	}
}
