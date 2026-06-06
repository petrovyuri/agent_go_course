// module-09-state/lesson-9-6/main.go

// Урок 9.6. Надёжность: таймауты, контекст, отмена.
// Контекст пронизывает весь граф: его получают все узлы. Если на запуск задан
// таймаут (context.WithTimeout) и узел уважает ctx.Done(), долгая операция будет
// прервана, а Invoke вернёт ошибку контекста. Здесь "медленный" узел ждёт либо
// завершения работы, либо отмены контекста. С коротким таймаутом он не успевает
// и прерывается; с достаточным — доходит до конца.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст и таймауты
	"fmt"       // вывод в консоль
	"log"       // log.Fatalf/Printf
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"syscall"   // коды сигналов
	"time"      // длительности и таймеры

	"github.com/cloudwego/eino-ext/devops" // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/compose"    // Graph, Lambda, Runnable
)

// runWithTimeout запускает граф с заданным таймаутом. cancel вызывается через
// defer, поэтому контекст всегда освобождается.
func runWithTimeout(ctx context.Context, r compose.Runnable[string, string], d time.Duration, in string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return r.Invoke(ctx, in)
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// "Медленный" узел: работает ~400 мс, но честно слушает отмену контекста.
	slow := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		select {
		case <-time.After(400 * time.Millisecond):
			return "готово: " + in, nil
		case <-ctx.Done():
			return "", ctx.Err() // контекст отменён или истёк таймаут
		}
	})

	g := compose.NewGraph[string, string]()
	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}
	add(g.AddLambdaNode("slow", slow))
	add(g.AddEdge(compose.START, "slow"))
	add(g.AddEdge("slow", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	runnable, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	// Короткий таймаут (150 мс): узел не успевает, запуск прерывается.
	if _, err := runWithTimeout(ctx, runnable, 150*time.Millisecond, "задача"); err != nil {
		fmt.Println("короткий таймаут — прервано:", err)
	}

	// Достаточный таймаут (1 с): узел успевает завершиться.
	out, err := runWithTimeout(ctx, runnable, time.Second, "задача")
	if err != nil {
		fmt.Println("неожиданная ошибка:", err)
		return
	}
	fmt.Println("достаточный таймаут —", out)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф. Ctrl+C — выход.
	log.Println("Граф собран. Откройте плагин EinoDev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
