// module-09-state/lesson-9-2/main.go

// Урок 9.2. Память диалога: история сообщений в состоянии.
// Кроме доступа из узла через ProcessState (урок 9.1), у состояния есть ещё два
// удобных хука на узле: StatePreHandler читает состояние и меняет ВХОД узла
// перед запуском, а StatePostHandler пишет ВЫХОД узла в состояние после. Здесь в
// состоянии лежит история сообщений: узел "remember" дописывает реплику
// (post-handler), а узел "recall" читает историю и отвечает с её учётом
// (pre-handler). Так состояние работает как память в пределах одного запуска.
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
	"github.com/cloudwego/eino/compose"    // Graph, состояние, хуки
	"github.com/cloudwego/eino/schema"     // Message
)

// chatState — состояние графа: история сообщений диалога.
type chatState struct {
	History []*schema.Message
}

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// Граф со стартовой историей: одно системное сообщение.
	g := compose.NewGraph[string, string](
		compose.WithGenLocalState(func(_ context.Context) *chatState {
			return &chatState{History: []*schema.Message{schema.SystemMessage("Ты ассистент.")}}
		}),
	)

	// Узел "remember": просто пропускает текст; настоящая работа — в post-handler,
	// который дописывает реплику пользователя в историю (запись в состояние).
	remember := compose.InvokableLambda(func(_ context.Context, in string) (string, error) {
		return in, nil
	})

	// Узел "recall": его pre-handler читает историю и подменяет вход на сводку,
	// поэтому сам узел уже видит, что лежит в памяти (чтение из состояния).
	recall := compose.InvokableLambda(func(_ context.Context, in string) (string, error) {
		return in, nil
	})

	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}

	// post-handler: после "remember" дописываем сообщение пользователя в историю.
	add(g.AddLambdaNode("remember", remember,
		compose.WithStatePostHandler(func(_ context.Context, out string, s *chatState) (string, error) {
			s.History = append(s.History, schema.UserMessage(out))
			return out, nil
		})))

	// pre-handler: перед "recall" читаем историю и формируем из неё вход узла.
	add(g.AddLambdaNode("recall", recall,
		compose.WithStatePreHandler(func(_ context.Context, _ string, s *chatState) (string, error) {
			last := s.History[len(s.History)-1].Content
			return fmt.Sprintf("в истории %d сообщений, последнее от пользователя: %q", len(s.History), last), nil
		})))

	add(g.AddEdge(compose.START, "remember"))
	add(g.AddEdge("remember", "recall"))
	add(g.AddEdge("recall", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	runnable, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	out, err := runnable.Invoke(ctx, "Париж — столица Франции")
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
