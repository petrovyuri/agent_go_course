// minicode/stage-06-final/mcpserver/main.go

// Небольшой MCP-сервер на Go. Он отдаёт два инструмента, которых у самого Mini
// Code нет: git_log (последние коммиты) и project_tree (дерево проекта). Сервер
// общается по stdio, поэтому Mini Code запускает его как отдельный процесс и
// подключается клиентом (см. mcp.go в родительской папке).
//
// Это и есть смысл MCP: инструмент живёт во внешнем сервере, а агент пользуется
// им через единый протокол, не зная, как сервер устроен внутри.
package main

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("project-tools", "1.0.0")

	// git_log — последние коммиты репозитория (git log --oneline).
	s.AddTool(
		mcp.NewTool("git_log",
			mcp.WithDescription("Последние коммиты git (история изменений проекта)"),
			mcp.WithNumber("count", mcp.Description("сколько коммитов показать (по умолчанию 5)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			n := req.GetInt("count", 5)
			if n < 1 || n > 50 {
				n = 5
			}
			// Фиксированная команда git, число коммитов — целое из диапазона.
			out, err := exec.CommandContext(ctx, "git", "log", "--oneline", "-n", strconv.Itoa(n)).CombinedOutput() //nolint:gosec // фиксированная команда git, аргумент — int
			if err != nil {
				return mcp.NewToolResultText("git недоступен или это не репозиторий: " + string(out)), nil //nolint:nilerr // ошибку git отдаём агенту текстом, а не как сбой инструмента
			}
			return mcp.NewToolResultText(string(out)), nil
		},
	)

	// project_tree — дерево файлов проекта (пропускает скрытые папки и vendor).
	s.AddTool(
		mcp.NewTool("project_tree",
			mcp.WithDescription("Дерево файлов проекта от текущей папки"),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText(projectTree(".")), nil
		},
	)

	if err := server.ServeStdio(s); err != nil {
		panic(err)
	}
}

// projectTree строит отступами дерево файлов от root, пропуская скрытые папки,
// vendor и node_modules.
func projectTree(root string) string {
	var b strings.Builder
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
			return fs.SkipDir
		}
		depth := strings.Count(rel, string(filepath.Separator))
		b.WriteString(strings.Repeat("  ", depth))
		if d.IsDir() {
			b.WriteString(name + "/\n")
		} else {
			b.WriteString(name + "\n")
		}
		return nil
	})
	return b.String()
}
