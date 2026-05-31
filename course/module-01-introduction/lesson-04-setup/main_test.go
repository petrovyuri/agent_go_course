package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchModels_OK проверяет, что fetchModels разбирает корректный ответ /api/tags
// и возвращает имена моделей. Используем httptest вместо живой Ollama, поэтому тест
// не требует запущенного сервера.
func TestFetchModels_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("неожиданный путь запроса: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen3.5"},{"name":"llama3.1"}]}`))
	}))
	defer srv.Close()

	models, err := fetchModels(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	want := []string{"qwen3.5", "llama3.1"}
	if len(models) != len(want) {
		t.Fatalf("ожидали %d моделей, получили %d: %v", len(want), len(models), models)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Errorf("модель %d: ожидали %q, получили %q", i, want[i], models[i])
		}
	}
}

// TestFetchModels_BadStatus проверяет, что ненулевой HTTP-статус превращается в ошибку.
func TestFetchModels_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := fetchModels(context.Background(), srv.URL); err == nil {
		t.Fatal("ожидали ошибку при статусе 500, получили nil")
	}
}

// TestFetchModels_NoServer проверяет, что обращение к неподнятому серверу возвращает
// ошибку, а не панику.
func TestFetchModels_NoServer(t *testing.T) {
	if _, err := fetchModels(context.Background(), "http://127.0.0.1:1"); err == nil {
		t.Fatal("ожидали ошибку соединения, получили nil")
	}
}
