// Урок 5.3. Своя функция как инструмент: InvokableTool, utils.InferTool и utils.NewTool.
// Берём обычную Go-функцию и превращаем её в инструмент, который умеет исполняться
// по строке JSON-аргументов (именно так его потом дёрнет ToolsNode).
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст
	"fmt"     // вывод в консоль
	"log"     // log.Fatalf — остановиться с понятной ошибкой
	"strings" // обработка строк

	"github.com/cloudwego/eino/components/tool/utils" // конструкторы инструментов
	"github.com/cloudwego/eino/schema"                // ToolInfo для ручной схемы
)

// upperArgs — параметры инструмента. Теги задают схему автоматически:
// json — имя параметра для модели, jsonschema:"required" — обязательность,
// jsonschema_description — человекочитаемое описание поля.
type upperArgs struct {
	Text string `json:"text" jsonschema:"required" jsonschema_description:"строка для преобразования"`
}

// repeatArgs — параметры второго инструмента (схему зададим вручную).
type repeatArgs struct {
	Text string `json:"text"`
	N    int    `json:"n"`
}

// toUpper и repeatText — функции-инструменты, вынесенные из main.
// Так их проще читать, переиспользовать и тестировать. ctx здесь не нужен,
// поэтому называем его _ (идиоматично для неиспользуемого параметра).
func toUpper(_ context.Context, in upperArgs) (string, error) {
	return strings.ToUpper(in.Text), nil
}

func repeatText(_ context.Context, in repeatArgs) (string, error) {
	return strings.Repeat(in.Text, in.N), nil
}

func main() {
	ctx := context.Background()

	// Способ 1 (рекомендуемый): InferTool выводит схему из тегов структуры.
	// Функцию передаём по имени — без анонимной обёртки.
	upper, err := utils.InferTool("to_upper", "Переводит строку в верхний регистр", toUpper)
	if err != nil {
		log.Fatalf("InferTool: %v", err)
	}

	// Инструмент исполняется по строке JSON-аргументов — это и есть его "интерфейс".
	out, err := upper.InvokableRun(ctx, `{"text":"привет"}`)
	if err != nil {
		log.Fatalf("исполнение to_upper: %v", err)
	}
	fmt.Println("to_upper:", out) // ПРИВЕТ

	// Способ 2: NewTool — когда схему удобнее задать вручную (как в уроке 5.2).
	info := &schema.ToolInfo{
		Name: "repeat",
		Desc: "Повторяет строку n раз",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {Type: schema.String, Desc: "строка", Required: true},
			"n":    {Type: schema.Integer, Desc: "сколько раз повторить", Required: true},
		}),
	}
	repeat := utils.NewTool(info, repeatText)

	out2, err := repeat.InvokableRun(ctx, `{"text":"go","n":3}`)
	if err != nil {
		log.Fatalf("исполнение repeat: %v", err)
	}
	fmt.Println("repeat:  ", out2) // gogogo
}
