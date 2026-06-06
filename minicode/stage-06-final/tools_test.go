// minicode/stage-06-final/tools_test.go

// Тесты инструментов Mini Code: безопасность (safePath), запись с подтверждением
// (write_file, edit_file) и allowlist команд (run_command). Подтверждение
// (confirm) в тестах подменяем заглушкой, чтобы не читать stdin. Без Ollama.
package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

// write создаёт файл с содержимым в текущей папке или останавливает тест.
func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("запись %s: %v", name, err)
	}
}

// allow подменяет confirm на ответ value на время теста.
func allow(t *testing.T, value bool) {
	t.Helper()
	prev := confirm
	confirm = func(string) bool { return value }
	t.Cleanup(func() { confirm = prev })
}

// TestSafePathRejects: секреты и выход за рабочую папку запрещены.
func TestSafePathRejects(t *testing.T) {
	for _, p := range []string{"../secret", ".env", ".env.local", ".ENV"} {
		if _, err := safePath(p); err == nil {
			t.Errorf("safePath(%q): ожидали отказ, ошибки нет", p)
		}
	}
}

// TestWriteFileConfirmed: при подтверждении файл создаётся.
func TestWriteFileConfirmed(t *testing.T) {
	t.Chdir(t.TempDir())
	allow(t, true)

	out, err := writeFile(context.Background(), writeFileArgs{Path: "a.txt", Content: "привет"})
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	if !strings.Contains(out, "записано") {
		t.Errorf("ожидали отчёт о записи, получили: %q", out)
	}
	data, err := os.ReadFile("a.txt")
	if err != nil || string(data) != "привет" {
		t.Errorf("файл записан неверно: %q (err=%v)", data, err)
	}

	// Запись .env запрещена даже при подтверждении.
	if _, err := writeFile(context.Background(), writeFileArgs{Path: ".env", Content: "x"}); err == nil {
		t.Error("ожидали отказ записи в .env, ошибки нет")
	}
}

// TestWriteFileDeclined: при отказе файл не создаётся.
func TestWriteFileDeclined(t *testing.T) {
	t.Chdir(t.TempDir())
	allow(t, false)

	out, err := writeFile(context.Background(), writeFileArgs{Path: "b.txt", Content: "данные"})
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	if out != "отменено пользователем" {
		t.Errorf("ожидали 'отменено пользователем', получили: %q", out)
	}
	if _, err := os.Stat("b.txt"); err == nil {
		t.Error("файл не должен был создаться при отказе")
	}
}

// TestEditFile: при подтверждении заменяет подстроку; если её нет — ошибка.
func TestEditFile(t *testing.T) {
	t.Chdir(t.TempDir())
	allow(t, true)
	write(t, "code.go", "var x = 1")

	if _, err := editFile(context.Background(), editFileArgs{Path: "code.go", Old: "x = 1", New: "x = 2"}); err != nil {
		t.Fatalf("editFile: %v", err)
	}
	data, _ := os.ReadFile("code.go")
	if string(data) != "var x = 2" {
		t.Errorf("замена неверна: %q", data)
	}

	if _, err := editFile(context.Background(), editFileArgs{Path: "code.go", Old: "нет", New: "y"}); err == nil {
		t.Error("ожидали ошибку при отсутствии подстроки")
	}
}

// TestRunCommandAllowlist: разрешены только команды go, остальное — отказ.
func TestRunCommandAllowlist(t *testing.T) {
	allow(t, true)
	ctx := context.Background()

	out, err := runCommand(ctx, runCmdArgs{Command: "go version"})
	if err != nil {
		t.Fatalf("runCommand(go version): %v", err)
	}
	if !strings.Contains(out, "go version") {
		t.Errorf("ожидали вывод версии go, получили: %q", out)
	}

	// Не-go команда отклоняется ещё до подтверждения.
	if _, err := runCommand(ctx, runCmdArgs{Command: "rm -rf /"}); err == nil {
		t.Error("ожидали отказ на не-go команду, ошибки нет")
	}
}
