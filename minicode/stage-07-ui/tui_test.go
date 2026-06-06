// minicode/stage-07-ui/tui_test.go

// Тесты терминального интерфейса: разбор нажатий (выход, сброс, отправка вопроса)
// и канальный механизм подтверждений. Агент и Ollama здесь не нужны.
package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/schema"
)

// newTestModel собирает модель в изолированной папке (своя session.json).
func newTestModel(t *testing.T) tuiModel {
	t.Helper()
	t.Chdir(t.TempDir())
	reqCh := make(chan string)
	respCh := make(chan bool)
	return newTUIModel(t.Context(), nil, reqCh, respCh)
}

// keyEnter — нажатие Enter.
func keyEnter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

// keyRune — нажатие одной печатной клавиши (например, "y").
func keyRune(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestHandleKey_Exit(t *testing.T) {
	m := newTestModel(t)
	m.input.SetValue("exit")
	_, cmd := m.handleKey(keyEnter())
	if cmd == nil {
		t.Fatal("ожидали команду выхода, получили nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ожидали tea.QuitMsg от команды выхода")
	}
}

func TestHandleKey_Reset(t *testing.T) {
	m := newTestModel(t)
	m.msgs = append(m.msgs, schema.UserMessage("первый вопрос"))
	m.input.SetValue("/reset")
	out, _ := m.handleKey(keyEnter())
	got := out.(tuiModel)
	if len(got.msgs) != 1 {
		t.Fatalf("после /reset ожидали 1 сообщение (system), получили %d", len(got.msgs))
	}
	if got.state != stIdle {
		t.Fatalf("после /reset ожидали состояние stIdle, получили %d", got.state)
	}
}

func TestHandleKey_SubmitQuestion(t *testing.T) {
	m := newTestModel(t)
	before := len(m.msgs)
	m.input.SetValue("что делает safePath")
	out, cmd := m.handleKey(keyEnter())
	got := out.(tuiModel)
	if got.state != stThinking {
		t.Fatalf("после вопроса ожидали состояние stThinking, получили %d", got.state)
	}
	if len(got.msgs) != before+1 {
		t.Fatalf("ожидали +1 сообщение пользователя, было %d стало %d", before, len(got.msgs))
	}
	if got.input.Value() != "" {
		t.Fatalf("поле ввода должно очиститься, осталось %q", got.input.Value())
	}
	if cmd == nil {
		t.Fatal("ожидали команду запуска агента, получили nil")
	}
}

func TestHandleKey_ConfirmYes(t *testing.T) {
	m := newTestModel(t)
	m.state = stConfirm
	out, cmd := m.handleKey(keyRune('y'))
	got := out.(tuiModel)
	if got.state != stThinking {
		t.Fatalf("после ответа y ожидали состояние stThinking, получили %d", got.state)
	}
	if cmd == nil {
		t.Fatal("ожидали команду отправки подтверждения, получили nil")
	}
}

func TestSendConfirm_DeliversAnswer(t *testing.T) {
	reqCh := make(chan string)
	respCh := make(chan bool)
	cmd := sendConfirm(t.Context(), reqCh, respCh, true)

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	if got := <-respCh; !got {
		t.Fatal("инструмент должен получить ответ true")
	}
	reqCh <- "следующее подтверждение" // разблокируем повторное ожидание
	msg := <-done
	req, ok := msg.(confirmRequestMsg)
	if !ok {
		t.Fatalf("ожидали confirmRequestMsg, получили %T", msg)
	}
	if req.prompt != "следующее подтверждение" {
		t.Fatalf("неверный текст запроса: %q", req.prompt)
	}
}

func TestView_ContainsWelcome(t *testing.T) {
	m := newTestModel(t)
	if v := m.View(); v == "" {
		t.Fatal("View вернул пустую строку")
	}
}
