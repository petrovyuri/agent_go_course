// module-11-rag-mcp/lesson-11-8-mcp/main_test.go

package main

import (
	"context"
	"testing"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
)

func TestStockReply(t *testing.T) {
	cases := []struct{ item, want string }{
		{"гайки", "На складе 42 шт.: гайки"},
		{"  Болты ", "На складе 17 шт.:   Болты "}, // регистр и пробелы не мешают поиску
		{"вертолёты", "Товар не найден на складе."},
	}
	for _, c := range cases {
		if got := stockReply(c.item); got != c.want {
			t.Errorf("stockReply(%q) = %q, ждали %q", c.item, got, c.want)
		}
	}
}

// TestMCPTools проверяет, что инструмент склада действительно отдаётся через MCP.
func TestMCPTools(t *testing.T) {
	ctx := context.Background()
	cli, err := newMCPClient(ctx)
	if err != nil {
		t.Fatalf("newMCPClient: %v", err)
	}
	tools, err := einomcp.GetTools(ctx, &einomcp.Config{Cli: cli})
	if err != nil {
		t.Fatalf("GetTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("ждали 1 инструмент, получили %d", len(tools))
	}
	info, err := tools[0].Info(ctx)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "stock" {
		t.Errorf("имя инструмента: ждали 'stock', получили %q", info.Name)
	}
}
