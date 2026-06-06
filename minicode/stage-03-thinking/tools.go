// minicode/stage-03-thinking/tools.go

// Инструменты Mini Code для работы с файлами: read_file, list_dir, grep.
// Все они только читают и не выходят за пределы рабочей папки. Чтение файлов
// с секретами (.env) запрещено — агент не должен видеть приватные данные.
package main

import (
	"context"       // контекст инструмента
	"fmt"           // сборка строк и ошибок
	"io/fs"         // обход дерева файлов
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
	Path    string `json:"path" jsonschema:"required" jsonschema_description:"путь к файлу или папке относительно рабочей папки; \".\" — весь проект"`
}

// grep ищет подстроку. Если путь — файл, ищет в нём; если папка — рекурсивно по
// всем её текстовым файлам (пропуская скрытые каталоги и .env). Каждое совпадение
// возвращается строкой вида "путь:номер: строка".
func grep(_ context.Context, in grepArgs) (string, error) {
	root, err := safePath(in.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть путь: %w", err)
	}

	var b strings.Builder
	// searchFile добавляет в b совпавшие строки одного файла (с его путём и номером).
	searchFile := func(file string) {
		data, errRead := os.ReadFile(file)
		if errRead != nil || len(data) > maxFileSize {
			return // пропускаем нечитаемое и слишком большое
		}
		lineNo := 0
		for line := range strings.SplitSeq(string(data), "\n") {
			lineNo++
			if strings.Contains(line, in.Pattern) {
				fmt.Fprintf(&b, "%s:%d: %s\n", file, lineNo, line)
			}
		}
	}

	if info.IsDir() {
		// Рекурсивный обход: пропускаем скрытые каталоги (.git и т.п.) и файлы-секреты.
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, errWalk error) error {
			if errWalk != nil {
				return errWalk // прервём обход, если запись недоступна
			}
			name := d.Name()
			if d.IsDir() {
				if p != root && strings.HasPrefix(name, ".") {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(strings.ToLower(name), ".env") {
				return nil
			}
			searchFile(p)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("обход папки: %w", err)
		}
	} else {
		searchFile(root)
	}

	if b.Len() == 0 {
		return "совпадений не найдено", nil
	}
	out := b.String()
	if len(out) > maxFileSize {
		out = out[:maxFileSize]
	}
	return out, nil
}
