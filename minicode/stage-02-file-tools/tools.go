// minicode/stage-02-file-tools/tools.go

// Инструменты Mini Code для работы с файлами: read_file, list_dir, grep.
// Все они только читают и не выходят за пределы рабочей папки. Чтение файлов
// с секретами (.env) запрещено — агент не должен видеть приватные данные.
package main

import (
	"context"       // контекст инструмента
	"fmt"           // сборка строк и ошибок
	"os"            // чтение файлов и папок
	"path/filepath" // безопасная работа с путями
	"strings"       // обработка текста
)

const maxFileSize = 64 * 1024 // максимум байт, который вернёт read_file

// safePath проверяет, что путь относительный, не ведёт наружу рабочей папки
// и не указывает на файл с секретами. Возвращает очищенный путь.
func safePath(path string) (string, error) {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("путь вне рабочей папки запрещён: %s", path)
	}
	base := strings.ToLower(filepath.Base(clean))
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return "", fmt.Errorf("чтение файлов с секретами запрещено: %s", path)
	}
	return clean, nil
}

// readFileArgs — параметр инструмента read_file.
type readFileArgs struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"путь к файлу относительно рабочей папки"`
}

// readFile возвращает содержимое файла (с ограничениями безопасности и размера).
func readFile(_ context.Context, in readFileArgs) (string, error) {
	path, err := safePath(in.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	if len(data) > maxFileSize {
		data = data[:maxFileSize]
	}
	return string(data), nil
}

// listDirArgs — параметр инструмента list_dir.
type listDirArgs struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"путь к папке относительно рабочей папки; \".\" — текущая"`
}

// listDir возвращает список файлов и папок каталога (папки помечены [dir]).
func listDir(_ context.Context, in listDirArgs) (string, error) {
	path, err := safePath(in.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать папку: %w", err)
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			b.WriteString("[dir] ")
		} else {
			b.WriteString("      ")
		}
		b.WriteString(e.Name())
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// grepArgs — параметры инструмента grep.
type grepArgs struct {
	Pattern string `json:"pattern" jsonschema:"required" jsonschema_description:"подстрока для поиска"`
	Path    string `json:"path" jsonschema:"required" jsonschema_description:"путь к файлу относительно рабочей папки"`
}

// grep ищет подстроку в файле и возвращает совпавшие строки с их номерами.
func grep(_ context.Context, in grepArgs) (string, error) {
	path, err := safePath(in.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	var b strings.Builder
	lineNo := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		lineNo++
		if strings.Contains(line, in.Pattern) {
			fmt.Fprintf(&b, "%d: %s\n", lineNo, line)
		}
	}
	if b.Len() == 0 {
		return "совпадений не найдено", nil
	}
	return b.String(), nil
}
