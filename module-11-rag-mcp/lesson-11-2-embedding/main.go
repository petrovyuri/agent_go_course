// module-11-rag-mcp/lesson-11-2-embedding/main.go

// Урок 11.2. Embedding через Ollama.
//
// Embedding (вектор) — это представление текста числами так, чтобы близкие по
// смыслу тексты давали близкие векторы. Это основа RAG: по вектору вопроса мы
// находим похожие куски базы знаний.
//
// Здесь мы превращаем несколько фраз в векторы через эмбеддер Ollama и считаем
// косинусную близость, чтобы увидеть: фразы про Go ближе друг к другу, чем к
// фразе про готовку.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделью qwen2.5-coder:1.5b (она умеет эмбеддинги).
package main

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/cloudwego/eino-ext/components/embedding/ollama" // эмбеддер Ollama
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	embedModel    = "qwen2.5-coder:1.5b"     // модель, считающая эмбеддинги
)

// cosine возвращает косинусную близость двух векторов: 1.0 — совпадают по
// направлению (очень похожи), около 0 — не связаны.
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

func main() {
	ctx := context.Background()

	// Создаём эмбеддер. Он ходит в Ollama по адресу BaseURL.
	embedder, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		BaseURL: ollamaBaseURL,
		Model:   embedModel,
	})
	if err != nil {
		log.Fatalf("не удалось создать эмбеддер: %v", err)
	}

	texts := []string{
		"В Go горутины — это лёгкие потоки.",
		"Язык Go компилируется в один бинарник.",
		"Борщ варят из свёклы и капусты.",
	}

	// EmbedStrings превращает срез текстов в срез векторов (по вектору на текст).
	vectors, err := embedder.EmbedStrings(ctx, texts)
	if err != nil {
		log.Fatalf("не удалось получить эмбеддинги: %v", err)
	}

	fmt.Printf("Текстов: %d, размерность вектора: %d\n\n", len(vectors), len(vectors[0]))

	fmt.Printf("Go vs Go:      %.3f  (про один язык — должно быть выше)\n", cosine(vectors[0], vectors[1]))
	fmt.Printf("Go vs борщ:    %.3f  (разные темы — должно быть ниже)\n", cosine(vectors[0], vectors[2]))
}
