// module-09-state/lesson-9-4/main.go

// Урок 9.4. Interrupt / Resume: human-in-the-loop.
// Иногда перед важным действием агент должен спросить человека. Узел может сам
// поставить паузу через StatefulInterrupt: он сохраняет своё состояние и отдаёт
// наружу info (что подтвердить), а запуск прерывается. Снаружи мы показываем
// вопрос человеку и возобновляем запуск с его решением через ResumeWithData. При
// повторном проходе узел узнаёт себя (GetInterruptState) и читает ответ человека
// (GetResumeContext). Это основа подтверждений (HITL), которые дадим Mini Code в
// модуле 10.
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
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/devops" // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/compose"    // Graph, interrupt/resume, checkpoint
	"github.com/cloudwego/eino/schema"     // регистрация типов для checkpoint
)

// approvalState — что узел запомнил на момент паузы (какое действие ждёт подтверждения).
type approvalState struct {
	Action string
}

// decision — ответ человека, который мы передаём при возобновлении.
type decision struct {
	Approved bool
}

// Типы, которые попадают в checkpoint, регистрируем для сериализации.
func init() {
	schema.Register[approvalState]()
	schema.Register[decision]()
}

// memStore — in-memory хранилище checkpoint-ов (нужно для паузы/возобновления).
type memStore struct{ m map[string][]byte }

func newMemStore() *memStore { return &memStore{m: make(map[string][]byte)} }
func (s *memStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	v, ok := s.m[id]
	return v, ok, nil
}
func (s *memStore) Set(_ context.Context, id string, cp []byte) error {
	s.m[id] = cp
	return nil
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	store := newMemStore()

	g := compose.NewGraph[string, string]()

	// Узел "approve": на первом проходе ставит паузу и просит подтверждение,
	// на возобновлении читает решение человека и действует по нему.
	approve := compose.InvokableLambda(func(ctx context.Context, action string) (string, error) {
		wasInterrupted, _, state := compose.GetInterruptState[*approvalState](ctx)
		if !wasInterrupted {
			// Первый проход: сохраняем действие и ставим паузу с вопросом наружу.
			return "", compose.StatefulInterrupt(ctx,
				"подтвердите действие: "+action,
				&approvalState{Action: action},
			)
		}

		// Возобновление: читаем ответ человека.
		_, hasData, d := compose.GetResumeContext[*decision](ctx)
		if hasData && d.Approved {
			return "выполнено: " + state.Action, nil
		}
		return "отменено: " + state.Action, nil
	})

	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}
	add(g.AddLambdaNode("approve", approve))
	add(g.AddEdge(compose.START, "approve"))
	add(g.AddEdge("approve", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	runnable, err := g.Compile(ctx, compose.WithCheckPointStore(store))
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	// Первый запуск: узел поставит паузу и попросит подтверждение.
	_, err = runnable.Invoke(ctx, "удалить config.yaml", compose.WithCheckPointID("act-1"))
	info, interrupted := compose.ExtractInterruptInfo(err)
	if !interrupted {
		log.Fatalf("ожидали паузу на подтверждение, но её нет (err=%v)", err)
	}
	fmt.Println("агент просит:", info.InterruptContexts[0].Info)

	// Здесь обычно спрашиваем человека. Сымитируем согласие.
	fmt.Println("человек отвечает: да")
	rCtx := compose.ResumeWithData(ctx, info.InterruptContexts[0].ID, &decision{Approved: true})

	result, err := runnable.Invoke(rCtx, "", compose.WithCheckPointID("act-1"))
	if err != nil {
		log.Fatalf("возобновление: %v", err)
	}
	fmt.Println("итог:", result)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф. Ctrl+C — выход.
	log.Println("Граф собран. Откройте плагин EinoDev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
