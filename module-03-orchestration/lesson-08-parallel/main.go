// Урок 3.8. Параллельные ветви и слияние данных.
// Несколько узлов обрабатывают один и тот же вход одновременно,
// а их результаты сливаются в общий map по именам веток.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст
	"fmt"       // вывод и форматирование
	"log"       // log.Fatalf — остановиться с понятной ошибкой
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"strings"   // обработка текста
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/devops" // debug-сервер для плагина EinoDev
	"github.com/cloudwego/eino/compose"    // оркестрация: Chain, Parallel
)

func main() {
	ctx := context.Background()

	// Поднимаем debug-сервер EinoDev ДО Compile — собранная цепочка попадёт в плагин.
	if err := devops.Init(ctx); err != nil {
		log.Fatalf("не удалось запустить EinoDev: %v", err)
	}

	// Параллельный блок: обе ветки получают ОДИН и тот же вход (строку)
	// и работают независимо. Результат каждой кладётся в map под её именем.
	parallel := compose.NewParallel().
		AddLambda("upper", compose.InvokableLambda(func(ctx context.Context, s string) (string, error) {
			return strings.ToUpper(s), nil
		})).
		AddLambda("length", compose.InvokableLambda(func(ctx context.Context, s string) (string, error) {
			return fmt.Sprintf("%d символов", len([]rune(s))), nil
		}))

	// Вход цепочки — строка, выход — map[string]any (слияние результатов веток).
	chain, err := compose.NewChain[string, map[string]any]().
		AppendParallel(parallel).
		Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать цепочку: %v", err)
	}

	out, err := chain.Invoke(ctx, "привет, агенты")
	if err != nil {
		log.Fatalf("ошибка выполнения: %v", err)
	}

	// В map лежат результаты обеих веток под ключами "upper" и "length".
	fmt.Println("upper :", out["upper"])
	fmt.Println("length:", out["length"])

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал цепочку.
	// Откройте панель Eino Dev в IDE и запускайте узлы с тестовым входом. Ctrl+C — выход.
	log.Println("EinoDev запущен. Откройте плагин Eino Dev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
