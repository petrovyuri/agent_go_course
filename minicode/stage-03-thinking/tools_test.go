// minicode/stage-03-thinking/tools_test.go

// Тесты файловых инструментов Mini Code. Проверяем прежде всего безопасность
// (safePath: запрет .env, выхода за рабочую папку и абсолютных путей) и базовое
// поведение read_file, list_dir и grep. Ollama для этих тестов не нужен.
package main

import (
	"context"
	"os"
	"path/filepath"
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

// TestSafePathAllows: обычные относительные пути проходят проверку.
func TestSafePathAllows(t *testing.T) {
	for _, p := range []string{"main.go", "sub/file.txt", "./a.go", "dir/../a.go"} {
		if _, err := safePath(p); err != nil {
			t.Errorf("safePath(%q): ожидали успех, получили ошибку: %v", p, err)
		}
	}
}

// TestSafePathRejects: выход за рабочую папку, секреты и абсолютные пути
// должны отвергаться.
func TestSafePathRejects(t *testing.T) {
	bad := []string{"../secret", "../../etc/passwd", ".env", ".env.local", ".ENV"}
	for _, p := range bad {
		if _, err := safePath(p); err == nil {
			t.Errorf("safePath(%q): ожидали отказ, ошибки нет", p)
		}
	}

	abs, err := filepath.Abs("x")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if _, err := safePath(abs); err == nil {
		t.Errorf("safePath(%q): ожидали отказ на абсолютный путь, ошибки нет", abs)
	}
}

// TestReadFile: читаем обычный файл, но отказываем в чтении .env.
func TestReadFile(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "hello.txt", "привет\nмир\n")
	write(t, ".env", "SECRET=1")

	ctx := context.Background()

	out, err := readFile(ctx, readFileArgs{Path: "hello.txt"})
	if err != nil {
		t.Fatalf("readFile(hello.txt): %v", err)
	}
	if !strings.Contains(out, "привет") {
		t.Errorf("ожидали содержимое файла, получили: %q", out)
	}

	if _, err := readFile(ctx, readFileArgs{Path: ".env"}); err == nil {
		t.Error("ожидали отказ при чтении .env, ошибки нет")
	}
}

// TestReadFileTruncates: слишком большой файл обрезается до maxFileSize.
func TestReadFileTruncates(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "big.txt", strings.Repeat("a", maxFileSize+100))

	out, err := readFile(context.Background(), readFileArgs{Path: "big.txt"})
	if err != nil {
		t.Fatalf("readFile(big.txt): %v", err)
	}
	if len(out) != maxFileSize {
		t.Errorf("ожидали обрезку до %d байт, получили %d", maxFileSize, len(out))
	}
}

// TestListDir: папки помечаются [dir], файлы перечисляются по имени.
func TestListDir(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "a.go", "x")
	if err := os.Mkdir("sub", 0o750); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	out, err := listDir(context.Background(), listDirArgs{Path: "."})
	if err != nil {
		t.Fatalf("listDir: %v", err)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "[dir] sub") {
		t.Errorf("ожидали a.go и [dir] sub, получили: %q", out)
	}
}

// TestGrep: совпадение возвращается с номером строки, иначе — понятное "не найдено".
func TestGrep(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "code.go", "package main\nfunc main() {}\n")

	ctx := context.Background()

	out, err := grep(ctx, grepArgs{Pattern: "func", Path: "code.go"})
	if err != nil {
		t.Fatalf("grep(func): %v", err)
	}
	if !strings.Contains(out, "2: func main()") {
		t.Errorf("ожидали строку с номером 2, получили: %q", out)
	}

	none, err := grep(ctx, grepArgs{Pattern: "нетвообще", Path: "code.go"})
	if err != nil {
		t.Fatalf("grep(нетвообще): %v", err)
	}
	if none != "совпадений не найдено" {
		t.Errorf("ожидали 'совпадений не найдено', получили: %q", none)
	}
}

// TestGrepDir: поиск по папке идёт рекурсивно, выводит путь файла и пропускает .env.
func TestGrepDir(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "a.go", "package main\nconst token = 1\n")
	if err := os.Mkdir("sub", 0o750); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	write(t, "sub/b.go", "var token = 2\n")
	write(t, ".env", "token=secret\n") // секрет grep по папке трогать не должен

	out, err := grep(context.Background(), grepArgs{Pattern: "token", Path: "."})
	if err != nil {
		t.Fatalf("grep(token, .): %v", err)
	}
	if !strings.Contains(out, "a.go:") || !strings.Contains(out, "b.go:") {
		t.Errorf("ожидали совпадения в a.go и sub/b.go, получили: %q", out)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("grep по папке не должен заглядывать в .env, получили: %q", out)
	}
}
