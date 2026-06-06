// minicode/stage-07-ui/tools.go

// Инструменты Mini Code. Чтение: read_file, list_dir, grep (как в модуле 8).
// Запись и команды: write_file, edit_file, run_command (новые в этом этапе).
// Все они проходят через safePath: нет выхода за рабочую папку и нет доступа к
// секретам (.env). Опасные инструменты (запись и команды) перед действием сами
// спрашивают подтверждение через confirm (см. main.go) — это human-in-the-loop.
// run_command разрешает только команды go и работает с таймаутом из контекста.
package main

import (
	"context"       // контекст инструмента (в т.ч. таймаут для команды)
	"fmt"           // сборка строк и ошибок
	"io/fs"         // обход дерева файлов для grep по папке
	"os"            // чтение и запись файлов
	"os/exec"       // запуск команды go
	"path/filepath" // безопасная работа с путями
	"strings"       // обработка текста
)

const maxFileSize = 64 * 1024 // максимум байт, который вернёт read_file/grep

// safePath проверяет, что путь относительный, не ведёт наружу рабочей папки
// и не указывает на файл с секретами. Возвращает очищенный путь.
func safePath(path string) (string, error) {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("путь вне рабочей папки запрещён: %s", path)
	}
	base := strings.ToLower(filepath.Base(clean))
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return "", fmt.Errorf("доступ к файлам с секретами запрещён: %s", path)
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

// grep ищет подстроку в файле или рекурсивно во всех файлах папки
// (пропуская скрытые каталоги и .env). Совпадение: "путь:номер: строка".
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
	searchFile := func(file string) {
		data, errRead := os.ReadFile(file)
		if errRead != nil || len(data) > maxFileSize {
			return
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
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, errWalk error) error {
			if errWalk != nil {
				return errWalk
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

// writeFileArgs — параметры инструмента write_file.
type writeFileArgs struct {
	Path    string `json:"path" jsonschema:"required" jsonschema_description:"путь к файлу относительно рабочей папки"`
	Content string `json:"content" jsonschema:"required" jsonschema_description:"новое содержимое файла"`
}

// writeFile создаёт или перезаписывает файл — после подтверждения пользователя.
func writeFile(_ context.Context, in writeFileArgs) (string, error) {
	path, err := safePath(in.Path)
	if err != nil {
		return "", err
	}
	if !confirm(fmt.Sprintf("записать %d байт в %s", len(in.Content), path)) {
		return "отменено пользователем", nil
	}
	// путь уже проверен safePath (нет выхода за рабочую папку и .env)
	if err := os.WriteFile(path, []byte(in.Content), 0o600); err != nil { //nolint:gosec // путь проверен safePath
		return "", fmt.Errorf("не удалось записать файл: %w", err)
	}
	return fmt.Sprintf("записано %d байт в %s", len(in.Content), path), nil
}

// editFileArgs — параметры инструмента edit_file.
type editFileArgs struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"путь к файлу относительно рабочей папки"`
	Old  string `json:"old" jsonschema:"required" jsonschema_description:"подстрока, которую нужно заменить"`
	New  string `json:"new" jsonschema:"required" jsonschema_description:"на что заменить"`
}

// editFile заменяет первое вхождение old на new в файле — после подтверждения.
func editFile(_ context.Context, in editFileArgs) (string, error) {
	path, err := safePath(in.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	if !strings.Contains(string(data), in.Old) {
		return "", fmt.Errorf("подстрока не найдена, замена не выполнена")
	}
	if !confirm("заменить подстроку в " + path) {
		return "отменено пользователем", nil
	}
	updated := strings.Replace(string(data), in.Old, in.New, 1)
	// путь уже проверен safePath (нет выхода за рабочую папку и .env)
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil { //nolint:gosec // путь проверен safePath
		return "", fmt.Errorf("не удалось записать файл: %w", err)
	}
	return "заменено первое вхождение в " + path, nil
}

// runCmdArgs — параметр инструмента run_command.
type runCmdArgs struct {
	Command string `json:"command" jsonschema:"required" jsonschema_description:"команда go, например: go build ./... или go test ./..."`
}

// runCommand запускает только команды go (allowlist) — после подтверждения, с таймаутом из контекста.
func runCommand(ctx context.Context, in runCmdArgs) (string, error) {
	fields := strings.Fields(in.Command)
	if len(fields) == 0 || fields[0] != "go" {
		return "", fmt.Errorf("разрешены только команды go (например: go build ./...)")
	}
	if !confirm("выполнить команду: " + in.Command) {
		return "отменено пользователем", nil
	}
	// Только команды go из allowlist; запуск уже подтверждён пользователем.
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...) //nolint:gosec // allowlist go + подтверждение пользователя
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("команда завершилась с ошибкой: %v\n%s", err, out), nil
	}
	if len(out) == 0 {
		return "команда выполнена без вывода", nil
	}
	return string(out), nil
}
