// minicode/stage-07-ui/mcp.go

// Подключение MCP-сервера к Mini Code. Наш сервер (папка mcpserver) запускается
// как отдельный процесс и общается по stdio. Мост mcp.GetTools из eino-ext
// превращает его инструменты в обычные инструменты Eino — агент пользуется ими
// так же, как своими. Чтобы подключить чужой сервер (например, по HTTP),
// меняется только конструктор клиента.
package main

import (
	"context"
	"fmt"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp" // мост MCP -> инструменты Eino
	"github.com/cloudwego/eino/components/tool"                 // BaseTool
	"github.com/mark3labs/mcp-go/client"                        // MCP-клиент
	"github.com/mark3labs/mcp-go/mcp"                           // типы протокола
)

// connectProjectMCP запускает наш MCP-сервер (go run ./mcpserver) как отдельный
// процесс по stdio и возвращает его инструменты как инструменты Eino. Клиент
// возвращаем, чтобы держать процесс живым, пока работает Mini Code.
func connectProjectMCP(ctx context.Context) ([]tool.BaseTool, *client.Client, error) {
	cli, err := client.NewStdioMCPClient("go", nil, "run", "./mcpserver")
	if err != nil {
		return nil, nil, fmt.Errorf("запуск MCP-сервера: %w", err)
	}
	if err := cli.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("старт MCP-клиента: %w", err)
	}
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		return nil, nil, fmt.Errorf("инициализация MCP: %w", err)
	}
	tools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: cli})
	if err != nil {
		return nil, nil, fmt.Errorf("инструменты MCP: %w", err)
	}
	return tools, cli, nil
}
