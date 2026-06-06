// module-09-state/lesson-9-7/main.go

// Урок 9.7. Практика: агент с подтверждением действий.
// Собираем вместе всё из модуля: состояние (журнал действий), checkpoint и
// interrupt/resume. Агент выполняет команды; безопасные — сразу, а опасные
// (удаление) ставит на паузу и просит подтверждение у человека. Состояние
// (журнал) переживает паузу: после возобновления в нём видны и шаги до паузы, и
// после. Это прямой прообраз human-in-the-loop в Mini Code (модуль 10).
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
	"strings"   // разбор команды и сборка журнала
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/devops" // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/compose"    // Graph, состояние, interrupt/resume
	"github.com/cloudwego/eino/schema"     // регистрация типов для checkpoint
)

// session — состояние запуска: журнал действий агента.
type session struct {
	Log []string
}

// confirmState — что агент запомнил на паузе (команду, ждущую подтверждения).
type confirmState struct {
	Cmd string
}

// decision — ответ человека на запрос подтверждения.
type decision struct {
	Approved bool
}

// Типы, попадающие в checkpoint, регистрируем для сериализации.
func init() {
	schema.Register[session]()
	schema.Register[confirmState]()
	schema.Register[decision]()
}

// logStep дописывает шаг в журнал состояния.
func logStep(ctx context.Context, step string) error {
	return compose.ProcessState(ctx, func(_ context.Context, s *session) error {
		s.Log = append(s.Log, step)
		return nil
	})
}

// finishWithLog возвращает результат вместе с накопленным журналом.
func finishWithLog(ctx context.Context, result string) (string, error) {
	var entries []string
	if err := compose.ProcessState(ctx, func(_ context.Context, s *session) error {
		entries = append(entries, s.Log...)
		return nil
	}); err != nil {
		return "", err
	}
	return result + " | журнал: " + strings.Join(entries, "; "), nil
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	store := &memStore{m: make(map[string][]byte)}

	g := compose.NewGraph[string, string](
		compose.WithGenLocalState(func(_ context.Context) *session { return &session{} }),
	)

	// Узел "run": безопасные команды выполняет сразу, опасные ставит на паузу.
	runCmd := compose.InvokableLambda(func(ctx context.Context, cmd string) (string, error) {
		// Возобновление после подтверждения: читаем команду из состояния и ответ человека.
		if wasInterrupted, _, st := compose.GetInterruptState[*confirmState](ctx); wasInterrupted {
			if _, hasData, d := compose.GetResumeContext[*decision](ctx); hasData && d.Approved {
				if err := logStep(ctx, "подтверждено, выполняю: "+st.Cmd); err != nil {
					return "", err
				}
				return finishWithLog(ctx, "выполнено: "+st.Cmd)
			}
			if err := logStep(ctx, "отклонено человеком"); err != nil {
				return "", err
			}
			return finishWithLog(ctx, "отменено: "+st.Cmd)
		}

		// Первый проход.
		if err := logStep(ctx, "получена команда: "+cmd); err != nil {
			return "", err
		}
		if strings.HasPrefix(cmd, "удалить") {
			if err := logStep(ctx, "опасная команда, прошу подтверждение"); err != nil {
				return "", err
			}
			return "", compose.StatefulInterrupt(ctx, "подтвердите: "+cmd, &confirmState{Cmd: cmd})
		}
		return finishWithLog(ctx, "выполнено: "+cmd)
	})

	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}
	add(g.AddLambdaNode("run", runCmd))
	add(g.AddEdge(compose.START, "run"))
	add(g.AddEdge("run", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	runnable, err := g.Compile(ctx, compose.WithCheckPointStore(store))
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	// Сценарий 1: безопасная команда — выполняется сразу, без паузы.
	safe, err := runnable.Invoke(ctx, "показать config.yaml", compose.WithCheckPointID("cmd-safe"))
	if err != nil {
		log.Fatalf("безопасная команда: %v", err)
	}
	fmt.Println("1)", safe)

	// Сценарий 2: опасная команда — пауза на подтверждение, затем возобновление.
	_, err = runnable.Invoke(ctx, "удалить config.yaml", compose.WithCheckPointID("cmd-danger"))
	info, interrupted := compose.ExtractInterruptInfo(err)
	if !interrupted {
		log.Fatalf("ожидали паузу на подтверждение (err=%v)", err)
	}
	fmt.Println("2) агент просит:", info.InterruptContexts[0].Info)
	fmt.Println("   человек отвечает: да")
	rCtx := compose.ResumeWithData(ctx, info.InterruptContexts[0].ID, &decision{Approved: true})
	danger, err := runnable.Invoke(rCtx, "", compose.WithCheckPointID("cmd-danger"))
	if err != nil {
		log.Fatalf("возобновление: %v", err)
	}
	fmt.Println("  ", danger)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф. Ctrl+C — выход.
	log.Println("Граф собран. Откройте плагин EinoDev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}

// memStore — in-memory хранилище checkpoint-ов.
type memStore struct{ m map[string][]byte }

func (s *memStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	v, ok := s.m[id]
	return v, ok, nil
}
func (s *memStore) Set(_ context.Context, id string, cp []byte) error {
	s.m[id] = cp
	return nil
}
