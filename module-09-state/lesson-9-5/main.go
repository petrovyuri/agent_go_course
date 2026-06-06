// module-09-state/lesson-9-5/main.go

// Урок 9.5. Обработка ошибок узлов и ретраи.
// Если узел вернул ошибку, граф прекращает работу и Invoke возвращает эту ошибку
// — её нужно обработать снаружи. Встроенных ретраев в Eino нет, поэтому повтор
// делаем сами: оборачиваем Invoke в цикл и повторяем при сбое. Здесь узел
// "нестабилен" — первые две попытки падает, на третьей срабатывает, и цикл
// ретраев доводит дело до конца.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context"   // контекст вызова
	"fmt"       // вывод и ошибки
	"log"       // log.Fatalf/Printf
	"os"        // сигналы ОС
	"os/signal" // ожидание Ctrl+C
	"syscall"   // коды сигналов

	"github.com/cloudwego/eino-ext/devops" // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/compose"    // Graph, Lambda
)

const maxRetries = 5 // сколько раз повторяем при сбое

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev до Compile — собранный граф будет виден в плагине.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	// Счётчик попыток держим в замыкании узла (без глобальных переменных).
	attempts := 0

	// Узел "fetch" нестабилен: первые две попытки возвращает ошибку.
	fetch := compose.InvokableLambda(func(_ context.Context, in string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", fmt.Errorf("временный сбой (попытка %d)", attempts)
		}
		return fmt.Sprintf("готово с %d-й попытки: %s", attempts, in), nil
	})

	g := compose.NewGraph[string, string]()
	var addErr error
	add := func(err error) {
		if addErr == nil && err != nil {
			addErr = err
		}
	}
	add(g.AddLambdaNode("fetch", fetch))
	add(g.AddEdge(compose.START, "fetch"))
	add(g.AddEdge("fetch", compose.END))
	if addErr != nil {
		log.Fatalf("сборка графа: %v", addErr)
	}

	runnable, err := g.Compile(ctx)
	if err != nil {
		log.Fatalf("компиляция: %v", err)
	}

	// Ретраи: повторяем Invoke, пока не получится или не кончатся попытки.
	var out string
	for try := 1; try <= maxRetries; try++ {
		out, err = runnable.Invoke(ctx, "загрузить данные")
		if err == nil {
			break
		}
		fmt.Printf("ретрай: попытка %d не удалась: %v\n", try, err)
	}
	if err != nil {
		log.Fatalf("исчерпаны ретраи (%d): %v", maxRetries, err)
	}
	fmt.Println(out)

	// EinoDev: держим процесс живым, чтобы плагин подключился и показал граф. Ctrl+C — выход.
	log.Println("Граф собран. Откройте плагин EinoDev и подключитесь. Ctrl+C — выход.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
