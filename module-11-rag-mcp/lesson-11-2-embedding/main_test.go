// module-11-rag-mcp/lesson-11-2-embedding/main_test.go

package main

import (
	"math"
	"testing"
)

func TestCosine(t *testing.T) {
	cases := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"одинаковое направление", []float64{1, 2, 3}, []float64{2, 4, 6}, 1},
		{"перпендикулярные", []float64{1, 0}, []float64{0, 1}, 0},
		{"противоположные", []float64{1, 0}, []float64{-1, 0}, -1},
		{"нулевой вектор", []float64{0, 0}, []float64{1, 1}, 0},
	}
	for _, c := range cases {
		if got := cosine(c.a, c.b); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: cosine = %v, ждали %v", c.name, got, c.want)
		}
	}
}
