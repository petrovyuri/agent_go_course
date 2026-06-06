// module-11-rag-mcp/lesson-11-5-rag-graph/main.go

// Урок 11.5. RAG-граф: retrieve -> prompt -> generate.
//
// Собираем полный RAG как граф Eino из трёх узлов:
//  1. retrieve — наш memStore ищет документы, похожие на вопрос;
//  2. prompt   — складываем найденные документы и вопрос в сообщения для модели;
//  3. generate — модель отвечает, опираясь на найденный контекст.
//
// Вопрос проносим между узлами через состояние графа (как в модуле 9): узел
// retrieve сохраняет его в state, а узел prompt достаёт оттуда.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделями qwen3.5 (ответ) и qwen2.5-coder:1.5b (эмбеддинги).
// Пока процесс работает, граф виден в EinoDev (devops.Init).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	embollama "github.com/cloudwego/eino-ext/components/embedding/ollama" // эмбеддер
	chatollama "github.com/cloudwego/eino-ext/components/model/ollama"    // чат-модель
	"github.com/cloudwego/eino-ext/devops"                                // сервер EinoDev
	"github.com/cloudwego/eino/components/model"                          // BaseChatModel
	"github.com/cloudwego/eino/compose"                                   // граф
	"github.com/cloudwego/eino/schema"                                    // Message, Document
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	embedModel    = "qwen2.5-coder:1.5b"     // модель для эмбеддингов
	chatModelName = "qwen3.5"                // модель для ответа
)

// knowledgeBase — маленькая база знаний о курсе.
var knowledgeBase = []string{
	"Этот курс учит создавать AI-агентов на Go с помощью фреймворка Eino и локальной модели через Ollama.",
	"ReAct — это цикл агента: рассуждение, вызов инструмента, наблюдение результата и так до готового ответа.",
	"Eino собирает агента как граф из узлов: модель, инструменты и ветвления.",
	"RAG добавляет агенту знания: по вектору вопроса находим похожие документы и кладём их в промпт.",
	"MCP — это открытый протокол: внешний сервер отдаёт инструменты, а агент их вызывает.",
	"Mini Code — учебный агент-кодер на Go, который читает и меняет файлы с подтверждением.",
}

// ragState — состояние графа: храним вопрос, чтобы пронести его от ретривера к промпту.
type ragState struct {
	question string
}

// buildRAGGraph собирает граф retrieve -> prompt -> generate.
func buildRAGGraph(ctx context.Context, store *memStore, chatModel model.BaseChatModel) (compose.Runnable[string, *schema.Message], error) {
	g := compose.NewGraph[string, *schema.Message](
		compose.WithGenLocalState(func(_ context.Context) *ragState { return &ragState{} }),
	)

	// Узел 1: ретривер. Перед поиском сохраняем вопрос в состояние.
	err := g.AddRetrieverNode("retrieve", store,
		compose.WithStatePreHandler(func(_ context.Context, q string, s *ragState) (string, error) {
			s.question = q
			return q, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("узел retrieve: %w", err)
	}

	// Узел 2: сборка промпта из найденных документов и вопроса из состояния.
	promptLambda := compose.InvokableLambda(func(ctx context.Context, docs []*schema.Document) ([]*schema.Message, error) {
		var question string
		if err := compose.ProcessState(ctx, func(_ context.Context, s *ragState) error {
			question = s.question
			return nil
		}); err != nil {
			return nil, err
		}

		var ctxText strings.Builder
		for i, d := range docs {
			ctxText.WriteString(strconv.Itoa(i + 1))
			ctxText.WriteString(". ")
			ctxText.WriteString(d.Content)
			ctxText.WriteString("\n")
		}

		system := schema.SystemMessage(
			"Ты помощник по курсу. Отвечай ТОЛЬКО на основе контекста ниже. " +
				"Если ответа в контексте нет, честно скажи об этом.\n\nКонтекст:\n" + ctxText.String())
		return []*schema.Message{system, schema.UserMessage(question)}, nil
	})
	if err := g.AddLambdaNode("prompt", promptLambda); err != nil {
		return nil, fmt.Errorf("узел prompt: %w", err)
	}

	// Узел 3: генерация ответа моделью.
	if err := g.AddChatModelNode("generate", chatModel); err != nil {
		return nil, fmt.Errorf("узел generate: %w", err)
	}

	// Рёбра: START -> retrieve -> prompt -> generate -> END.
	for _, e := range [][2]string{
		{compose.START, "retrieve"},
		{"retrieve", "prompt"},
		{"prompt", "generate"},
		{"generate", compose.END},
	} {
		if err := g.AddEdge(e[0], e[1]); err != nil {
			return nil, fmt.Errorf("ребро %s->%s: %w", e[0], e[1], err)
		}
	}

	return g.Compile(ctx)
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev — граф RAG виден в плагине, пока работает процесс.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	embedder, err := embollama.NewEmbedder(ctx, &embollama.EmbeddingConfig{
		BaseURL: ollamaBaseURL,
		Model:   embedModel,
	})
	if err != nil {
		log.Fatalf("не удалось создать эмбеддер: %v", err)
	}

	chatModel, err := chatollama.NewChatModel(ctx, &chatollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    chatModelName,
		Thinking: &chatollama.ThinkValue{Value: false},
	})
	if err != nil {
		log.Fatalf("не удалось создать чат-модель: %v", err)
	}

	// Индексируем базу знаний.
	store := newMemStore(embedder)
	docs := make([]*schema.Document, len(knowledgeBase))
	for i, text := range knowledgeBase {
		docs[i] = &schema.Document{Content: text}
	}
	if _, err := store.Store(ctx, docs); err != nil {
		log.Fatalf("не удалось проиндексировать документы: %v", err)
	}

	rag, err := buildRAGGraph(ctx, store, chatModel)
	if err != nil {
		log.Fatalf("не удалось собрать RAG-граф: %v", err)
	}

	question := "Что такое RAG и зачем он агенту?"
	answer, err := rag.Invoke(ctx, question)
	if err != nil {
		log.Fatalf("ошибка RAG-графа: %v", err)
	}

	fmt.Println("Вопрос:", question)
	fmt.Println("Ответ: ", answer.Content)

	// Держим процесс, чтобы успеть посмотреть граф в EinoDev. Выход по Ctrl+C.
	log.Println("Граф собран. Откройте EinoDev (адрес в строке start debug http server). Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
