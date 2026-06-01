// Урок 3.6. Ветвление: AddBranch и условные переходы.
// Граф выбирает один из двух путей в зависимости от входа.
// Намеренно без модели — так логика ветвления видна как на ладони.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст
	"fmt"       // вывод в консоль
	"log"       // log.Fatalf — остановиться с понятной ошибкой
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/devops" // debug-сервер для плагина EinoDev
	"github.com/cloudwego/eino/compose"    // оркестрация: Graph, NewGraphBranch
)

func main() {
	ctx := context.Background()

	// Поднимаем debug-сервер EinoDev ДО Compile — собранный граф попадёт в плагин.
	if err := devops.Init(ctx); err != nil {
		log.Fatalf("не удалось запустить EinoDev: %v", err)
	}

	// Граф: вход — строка (вопрос), выход — строка (помеченный вопрос).
	g := compose.NewGraph[string, string]()

	// Узел-вход: просто пропускает строку дальше (его выход читает ветвление).
	_ = g.AddLambdaNode("intake", compose.InvokableLambda(func(ctx context.Context, q string) (string, error) {
		return q, nil
	}))

	// Два узла-обработчика — по одному на ветку.
	_ = g.AddLambdaNode("short", compose.InvokableLambda(func(ctx context.Context, q string) (string, error) {
		return "короткий вопрос: " + q, nil
	}))
	_ = g.AddLambdaNode("long", compose.InvokableLambda(func(ctx context.Context, q string) (string, error) {
		return "длинный вопрос: " + q, nil
	}))

	_ = g.AddEdge(compose.START, "intake")

	// Ветвление: функция-условие смотрит на выход узла intake и возвращает
	// имя следующего узла. Второй аргумент — множество возможных целей.
	branch := compose.NewGraphBranch(
		func(ctx context.Context, q string) (string, error) {
			if len([]rune(q)) < 20 {
				return "short", nil
			}
			return "long", nil
		},
		map[string]bool{"short": true, "long": true},
	)
	_ = g.AddBranch("intake", branch)

	// Обе ветки ведут к выходу графа.
	_ = g.AddEdge("short", compose.END)
	_ = g.AddEdge("long", compose.END)

	runner, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать граф: %v", err)
	}

	// Прогоним два разных входа — увидим, что сработали разные ветки.
	for _, q := range []string{"Что такое Go?", "Объясни, как устроены каналы и горутины в Go"} {
		out, err := runner.Invoke(ctx, q)
		if err != nil {
			log.Fatalf("ошибка выполнения графа: %v", err)
		}
		fmt.Println(out)
	}

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф.
	// Откройте панель Eino Dev в IDE и запускайте узлы с тестовым входом. Ctrl+C — выход.
	log.Println("EinoDev запущен. Откройте плагин Eino Dev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
