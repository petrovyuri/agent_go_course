// module-09-state/lesson-9-3/main.go

// Урок 9.3. Checkpoints: сохранение и восстановление прогресса.
// Граф можно прервать и сохранить его состояние во внешнее хранилище
// (CheckPointStore), а позже продолжить с того же места. Это нужно для долгих
// задач и для human-in-the-loop. Здесь граф прерывается перед вторым узлом
// (WithInterruptBeforeNodes), прогресс попадает в in-memory store, а затем мы
// возобновляем запуск (Resume) — и он доходит до конца, не теряя состояния.
// Тип состояния регистрируем через schema.Register — иначе его не сериализовать.
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
	"github.com/cloudwego/eino/compose"    // Graph, checkpoints, interrupt
	"github.com/cloudwego/eino/schema"     // регистрация типа состояния
)

// progressState — состояние графа: журнал выполненных шагов.
type progressState struct {
	Done []string
}

// Регистрируем тип состояния для сериализации в checkpoint (обязательно).
func init() {
	schema.Register[progressState]()
}

// memStore — простейшее in-memory хранилище checkpoint-ов (Get/Set по id).
type memStore struct {
	m map[string][]byte
}

func newMemStore() *memStore { return &memStore{m: make(map[string][]byte)} }

func (s *memStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	v, ok := s.m[id]
	return v, ok, nil
}

func (s *memStore) Set(_ context.Context, id string, cp []byte) error {
	s.m[id] = cp
	return nil
}

// logStep дописывает шаг в состояние.
func logStep(ctx context.Context, step string) error {
	return compose.ProcessState(ctx, func(_ context.Context, s *progressState) error {
		s.Done = append(s.Done, step)
		return nil
	})
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	store := newMemStore()

	g := compose.NewGraph[string, string](
		compose.WithGenLocalState(func(_ context.Context) *progressState {
			return &progressState{}
		}),
	)

	// step1 отмечает свой шаг в состоянии.
	step1 := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		if err := logStep(ctx, "шаг1"); err != nil {
			return "", err
		}
		return in + ">1", nil
	})
	// step2 отмечает свой шаг и возвращает итог вместе с журналом из состояния.
	step2 := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		if err := logStep(ctx, "шаг2"); err != nil {
			return "", err
		}
		var done []string
		if err := compose.ProcessState(ctx, func(_ context.Context, s *progressState) error {
			done = append(done, s.Done...)
			return nil
		}); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s>2 (прогресс: %v)", in, done), nil
	})

	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}
	add(g.AddLambdaNode("step1", step1))
	add(g.AddLambdaNode("step2", step2))
	add(g.AddEdge(compose.START, "step1"))
	add(g.AddEdge("step1", "step2"))
	add(g.AddEdge("step2", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	// Компилируем с хранилищем checkpoint-ов и прерыванием перед step2.
	runnable, err := g.Compile(ctx,
		compose.WithCheckPointStore(store),
		compose.WithInterruptBeforeNodes([]string{"step2"}),
	)
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	// Первый запуск: дойдёт до step1 и прервётся перед step2.
	_, err = runnable.Invoke(ctx, "старт", compose.WithCheckPointID("job-1"))
	info, interrupted := compose.ExtractInterruptInfo(err)
	if !interrupted {
		log.Fatalf("ожидали прерывание, но его нет (err=%v)", err)
	}
	fmt.Println("прервано перед узлами:", info.BeforeNodes)
	if _, ok, _ := store.Get(ctx, "job-1"); ok {
		fmt.Println("checkpoint сохранён в хранилище: job-1")
	}

	// Возобновляем тот же checkpoint — граф продолжит со step2 с сохранённым прогрессом.
	rCtx := compose.Resume(ctx, info.InterruptContexts[0].ID)
	result, err := runnable.Invoke(rCtx, "старт", compose.WithCheckPointID("job-1"))
	if err != nil {
		log.Fatalf("возобновление: %v", err)
	}
	fmt.Println("после возобновления:", result)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф. Ctrl+C — выход.
	log.Println("Граф собран. Откройте плагин EinoDev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
