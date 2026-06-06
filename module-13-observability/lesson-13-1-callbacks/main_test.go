// module-13-observability/lesson-13-1-callbacks/main_test.go

package main

import (
	"context"
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

// TestTraceHandlerBuilds проверяет, что обработчик callbacks собирается.
func TestTraceHandlerBuilds(t *testing.T) {
	if traceHandler() == nil {
		t.Error("traceHandler вернул nil")
	}
}
