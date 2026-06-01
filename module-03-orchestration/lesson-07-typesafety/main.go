// Урок 3.7. Типобезопасность графа: дженерики NewGraph[I, O].
// Граф знает типы своего входа, выхода и каждого узла. Несовместимость
// ловится при Compile, а не в рантайме.
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
	"github.com/cloudwego/eino/compose"    // оркестрация: Graph
)

// Order — типизированный вход графа. Никаких map[string]any: компилятор
// проверит поля за нас.
type Order struct {
	Item string // что заказали
	Qty  int    // сколько
}

func main() {
	ctx := context.Background()

	// Поднимаем debug-сервер EinoDev ДО Compile — собранный граф попадёт в плагин.
	if err := devops.Init(ctx); err != nil {
		log.Fatalf("не удалось запустить EinoDev: %v", err)
	}

	// Вход графа — Order, выход — string. Эти типы — часть сигнатуры графа.
	g := compose.NewGraph[Order, string]()

	// Узел 1: Order -> string (краткое описание заказа).
	_ = g.AddLambdaNode("describe", compose.InvokableLambda(func(ctx context.Context, o Order) (string, error) {
		return fmt.Sprintf("%d × %s", o.Qty, o.Item), nil
	}))

	// Узел 2: string -> string (оформляем как чек). Его вход (string) совпадает
	// с выходом предыдущего узла — иначе Compile вернул бы ошибку.
	_ = g.AddLambdaNode("receipt", compose.InvokableLambda(func(ctx context.Context, s string) (string, error) {
		return "Чек: " + strings.ToUpper(s), nil
	}))

	_ = g.AddEdge(compose.START, "describe")
	_ = g.AddEdge("describe", "receipt")
	_ = g.AddEdge("receipt", compose.END)

	runner, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать граф: %v", err)
	}

	// Передаём типизированный Order — компилятор не даст передать что-то другое.
	out, err := runner.Invoke(ctx, Order{Item: "кофе", Qty: 3})
	if err != nil {
		log.Fatalf("ошибка выполнения графа: %v", err)
	}

	fmt.Println(out) // Чек: 3 × КОФЕ

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф.
	// Откройте панель Eino Dev в IDE и запускайте узлы с тестовым входом. Ctrl+C — выход.
	log.Println("EinoDev запущен. Откройте плагин Eino Dev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
