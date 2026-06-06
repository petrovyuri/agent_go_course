// Урок 5.2. Описание инструмента: schema.ToolInfo и схема параметров.
// Здесь мы только ОПИСЫВАЕМ инструмент (имя, назначение, параметры) — это то,
// что увидит модель. Исполнять его научимся в уроке 5.3.
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
package main

import (
	"context" // контекст (понадобится дальше)
	"fmt"     // вывод в консоль
	"log"     // log.Fatalf — остановиться с понятной ошибкой

	"github.com/cloudwego/eino/schema" // ToolInfo и ParameterInfo
)

func main() {
	_ = context.Background() // контекст ещё не нужен, но пусть пакет будет под рукой

	// ToolInfo — это "паспорт" инструмента: имя, описание и параметры.
	// Модель читает его и решает, когда и с какими аргументами вызвать инструмент.
	info := &schema.ToolInfo{
		Name: "add",
		Desc: "Складывает два целых числа и возвращает сумму",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"a": {Type: schema.Integer, Desc: "первое слагаемое", Required: true},
			"b": {Type: schema.Integer, Desc: "второе слагаемое", Required: true},
		}),
	}

	fmt.Println("Имя:      ", info.Name)
	fmt.Println("Описание: ", info.Desc)

	// Так описание выглядит в JSON — примерно это и уходит модели вместе с запросом.
	data, err := info.MarshalJSON()
	if err != nil {
		log.Fatalf("сериализация ToolInfo: %v", err)
	}
	fmt.Println("JSON-схема инструмента:")
	fmt.Println(string(data))
}
