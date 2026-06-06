// module-09-state/lesson-9-1/main.go

// Урок 9.1. Состояние графа: WithGenLocalState и ProcessState.
// Состояние — это общая на один запуск (Invoke) структура, которую видят все
// узлы графа. Создаётся фабрикой через WithGenLocalState, а из узлов к нему
// безопасно обращаются через ProcessState (он берёт мьютекс на время доступа).
// Здесь каждый узел дописывает свой шаг в общий журнал, а финальный узел его
// возвращает — видно, что состояние общее для всех узлов.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст вызова
	"fmt"       // вывод в консоль
	"log"       // log.Fatalf/Printf
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"strings"   // сборка журнала в строку
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/devops" // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/compose"    // Graph, состояние, Lambda
)

// auditState — состояние графа: журнал пройденных шагов.
type auditState struct {
	Steps []string
}

// logStep дописывает шаг в журнал состояния (потокобезопасно через ProcessState).
func logStep(ctx context.Context, step string) error {
	return compose.ProcessState(ctx, func(_ context.Context, s *auditState) error {
		s.Steps = append(s.Steps, step)
		return nil
	})
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// Граф со состоянием: фабрика создаёт пустой журнал на каждый запуск.
	g := compose.NewGraph[string, string](
		compose.WithGenLocalState(func(_ context.Context) *auditState {
			return &auditState{}
		}),
	)

	// Узел "validate": отмечает шаг в состоянии и пропускает ввод дальше.
	validate := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		if err := logStep(ctx, "проверка ввода"); err != nil {
			return "", err
		}
		return in, nil
	})

	// Узел "process": отмечает шаг и преобразует ввод.
	process := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		if err := logStep(ctx, "обработка"); err != nil {
			return "", err
		}
		return strings.ToUpper(in), nil
	})

	// Узел "report": читает журнал из состояния и собирает итоговый отчёт.
	report := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		if err := logStep(ctx, "отчёт"); err != nil {
			return "", err
		}
		var steps []string
		err := compose.ProcessState(ctx, func(_ context.Context, s *auditState) error {
			steps = append(steps, s.Steps...)
			return nil
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("результат: %s; шаги: %s", in, strings.Join(steps, " -> ")), nil
	})

	// Собираем линейный граф: validate -> process -> report.
	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}
	add(g.AddLambdaNode("validate", validate))
	add(g.AddLambdaNode("process", process))
	add(g.AddLambdaNode("report", report))
	add(g.AddEdge(compose.START, "validate"))
	add(g.AddEdge("validate", "process"))
	add(g.AddEdge("process", "report"))
	add(g.AddEdge("report", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	runnable, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	out, err := runnable.Invoke(ctx, "привет")
	if err != nil {
		log.Fatalf("запуск: %v", err)
	}
	fmt.Println(out)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф. Ctrl+C — выход.
	log.Println("Граф собран. Откройте плагин EinoDev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
