// module-07-react/lesson-7-7/tools_test.go

// Тесты инструментов агента-исследователя. Проверяем детерминированную логику
// (поиск по справочнику, сложение и ветку "нет данных") без обращения к Ollama.
package main

import (
	"context"
	"testing"
)

// TestPopulation: известная страна возвращает число, неизвестная — ошибку.
func TestPopulation(t *testing.T) {
	ctx := context.Background()

	got, err := population(ctx, countryArg{Country: "Франция"})
	if err != nil {
		t.Fatalf("population(Франция): неожиданная ошибка: %v", err)
	}
	if got != 68 {
		t.Errorf("population(Франция) = %d, ожидали 68", got)
	}

	if _, err := population(ctx, countryArg{Country: "Атлантида"}); err == nil {
		t.Error("population(Атлантида): ожидали ошибку про отсутствие данных, её нет")
	}
}

// TestCapital: известная страна возвращает столицу, неизвестная — ошибку.
func TestCapital(t *testing.T) {
	ctx := context.Background()

	got, err := capital(ctx, countryArg{Country: "Япония"})
	if err != nil {
		t.Fatalf("capital(Япония): неожиданная ошибка: %v", err)
	}
	if got != "Токио" {
		t.Errorf("capital(Япония) = %q, ожидали Токио", got)
	}

	if _, err := capital(ctx, countryArg{Country: "Атлантида"}); err == nil {
		t.Error("capital(Атлантида): ожидали ошибку про отсутствие данных, её нет")
	}
}

// TestAdd: сложение возвращает сумму.
func TestAdd(t *testing.T) {
	got, err := add(context.Background(), addArgs{A: 68, B: 84})
	if err != nil {
		t.Fatalf("add: неожиданная ошибка: %v", err)
	}
	if got != 152 {
		t.Errorf("add(68, 84) = %d, ожидали 152", got)
	}
}
