// module-13-observability/lesson-13-3-langfuse/main_test.go

package main

import (
	"context"
	"os"
	"testing"
)

func TestAdd(t *testing.T) {
	got, err := add(context.Background(), addArgs{A: 7, B: 5})
	if err != nil {
		t.Fatalf("add вернул ошибку: %v", err)
	}
	if got != 12 {
		t.Errorf("7 + 5: ждали 12, получили %d", got)
	}
}

// TestLoadEnvFile проверяет, что .env читается: комментарии и пустые строки
// пропускаются, кавычки и пробелы вокруг = снимаются.
func TestLoadEnvFile(t *testing.T) {
	t.Chdir(t.TempDir())
	content := "# комментарий\nLF_TEST_KEY = \"hello\"\nкривая строка без равно\n"
	if err := os.WriteFile(".env", []byte(content), 0o600); err != nil {
		t.Fatalf("не удалось создать .env: %v", err)
	}
	if err := os.Unsetenv("LF_TEST_KEY"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	loadEnvFile()
	if got := os.Getenv("LF_TEST_KEY"); got != "hello" {
		t.Errorf("из .env ждали 'hello', получили %q", got)
	}
}
