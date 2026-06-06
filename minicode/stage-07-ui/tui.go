// minicode/stage-07-ui/tui.go

// Терминальный интерфейс (TUI) Mini Code в духе Claude Code: рамка-приветствие,
// тёплый терракотовый акцент, поле ввода в рамке снизу и спиннер во время работы
// агента. Построен на charmbracelet (Bubble Tea + Lip Gloss + Bubbles).
//
// Про concurrency (важно): react.Agent.Generate — блокирующий и долгий, поэтому
// он крутится в отдельной горутине через tea.Cmd и возвращает результат
// сообщением. Подтверждения опасных действий (write_file/run_command) приходят из
// инструментов по каналу: инструмент шлёт текст запроса, TUI рисует "(y/n)" и
// отправляет ответ обратно — так human-in-the-loop работает и в графическом виде.
// Все горутины-команды завершаются по отмене контекста (defer cancel в runTUI),
// поэтому утечек нет.
package main

import (
	"context" // таймаут хода и отмена фоновых команд
	"strings" // сборка строк интерфейса

	"github.com/charmbracelet/bubbles/spinner"   // анимация ожидания
	"github.com/charmbracelet/bubbles/textinput" // поле ввода
	tea "github.com/charmbracelet/bubbletea"     // каркас TUI
	"github.com/charmbracelet/lipgloss"          // стили и рамки
	"github.com/cloudwego/eino/flow/agent/react" // агент
	"github.com/cloudwego/eino/schema"           // сообщения
)

// Фирменные цвета и стили в духе Claude Code (тёплый терракотовый акцент).
var (
	accentColor = lipgloss.Color("#D97757") // акцент Claude
	dimColor    = lipgloss.Color("#8A8A8A") // приглушённый серый
	userColor   = lipgloss.Color("#7AA2F7") // реплики пользователя

	titleStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	boxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accentColor).Padding(0, 1)
	inputStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dimColor).Padding(0, 1)
	dimStyle     = lipgloss.NewStyle().Foreground(dimColor)
	userStyle    = lipgloss.NewStyle().Foreground(userColor).Bold(true)
	answerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6E6E6"))
	confirmStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
)

// tuiState — что сейчас делает интерфейс.
type tuiState int

const (
	stIdle     tuiState = iota // ждём ввод пользователя
	stThinking                 // агент работает (крутим спиннер)
	stConfirm                  // ждём y/n на подтверждение опасного действия
)

// agentReplyMsg — агент закончил ход (ответ или ошибка).
type agentReplyMsg struct {
	answer *schema.Message
	err    error
}

// confirmRequestMsg — инструмент просит подтверждение перед опасным действием.
type confirmRequestMsg struct {
	prompt string
}

// tuiModel — состояние интерфейса (модель Bubble Tea).
type tuiModel struct {
	ctx     context.Context   // контекст приложения (отменяется при выходе)
	agent   *react.Agent      // тот же агент, что и в REPL
	input   textinput.Model   // поле ввода
	spin    spinner.Model     // анимация ожидания
	history []string          // отрисованные реплики (лента диалога)
	msgs    []*schema.Message // история для агента (память сессии)
	state   tuiState          // текущее состояние
	reqCh   chan string       // инструмент -> TUI: запрос подтверждения
	respCh  chan bool         // TUI -> инструмент: ответ y/n
	width   int               // ширина терминала
}

// newTUIModel собирает модель: поле ввода, спиннер и память сессии.
func newTUIModel(ctx context.Context, ag *react.Agent, reqCh chan string, respCh chan bool) tuiModel {
	in := textinput.New()
	in.Placeholder = "Спросите про код проекта... (exit — выход)"
	in.Prompt = "› "
	in.Focus()
	in.CharLimit = 2000

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = titleStyle

	return tuiModel{
		ctx:    ctx,
		agent:  ag,
		input:  in,
		spin:   sp,
		msgs:   loadSession(),
		state:  stIdle,
		reqCh:  reqCh,
		respCh: respCh,
	}
}

// Init запускает мигание курсора и подписку на запросы подтверждения.
func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, waitConfirm(m.ctx, m.reqCh))
}

// Update обрабатывает события: ввод, ответ агента, запрос подтверждения, тики спиннера.
func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = msg.Width - 6
		return m, nil

	case spinner.TickMsg:
		if m.state == stThinking || m.state == stConfirm {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case confirmRequestMsg:
		m.state = stConfirm
		m.history = append(m.history, confirmStyle.Render("⚠ "+msg.prompt+" (y/n)"))
		return m, m.spin.Tick

	case agentReplyMsg:
		m.state = stIdle
		if msg.err != nil {
			m.history = append(m.history, dimStyle.Render("ошибка: "+msg.err.Error()))
		} else {
			m.msgs = append(m.msgs, msg.answer)
			m.history = append(m.history, answerStyle.Render(strings.TrimSpace(msg.answer.Content)))
			saveSession(m.msgs)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleKey разбирает нажатия с учётом текущего состояния (ввод, подтверждение, выход).
func (m tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		saveSession(m.msgs)
		return m, tea.Quit
	}

	// Режим подтверждения: ждём y/n на опасное действие.
	if m.state == stConfirm {
		switch msg.String() {
		case "y", "Y", "д", "Д":
			m.state = stThinking
			m.history = append(m.history, dimStyle.Render("  → да"))
			return m, tea.Batch(sendConfirm(m.ctx, m.reqCh, m.respCh, true), m.spin.Tick)
		case "n", "N", "н", "Н", "esc":
			m.state = stThinking
			m.history = append(m.history, dimStyle.Render("  → нет"))
			return m, tea.Batch(sendConfirm(m.ctx, m.reqCh, m.respCh, false), m.spin.Tick)
		}
		return m, nil
	}

	// Обычный режим: Enter отправляет вопрос агенту.
	if m.state == stIdle && msg.Type == tea.KeyEnter {
		text := strings.TrimSpace(m.input.Value())
		switch text {
		case "":
			return m, nil
		case "exit":
			saveSession(m.msgs)
			return m, tea.Quit
		case "/reset":
			m.msgs = []*schema.Message{schema.SystemMessage(systemPrompt)}
			saveSession(m.msgs)
			m.history = append(m.history, dimStyle.Render("история очищена"))
			m.input.SetValue("")
			return m, nil
		}
		m.history = append(m.history, userStyle.Render("› "+text))
		m.msgs = append(m.msgs, schema.UserMessage(text))
		m.input.SetValue("")
		m.state = stThinking
		return m, tea.Batch(m.runAgent(), m.spin.Tick)
	}

	// Иначе — обычное редактирование поля ввода.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// runAgent запускает ход агента в отдельной горутине (Generate блокирующий).
func (m tuiModel) runAgent() tea.Cmd {
	ctx, ag, msgs := m.ctx, m.agent, m.msgs
	return func() tea.Msg {
		genCtx, cancel := context.WithTimeout(ctx, turnTimeout)
		defer cancel()
		ans, err := ag.Generate(genCtx, msgs)
		return agentReplyMsg{answer: ans, err: err}
	}
}

// waitConfirm ждёт запрос подтверждения от инструмента или отмену контекста.
func waitConfirm(ctx context.Context, reqCh chan string) tea.Cmd {
	return func() tea.Msg {
		select {
		case prompt := <-reqCh:
			return confirmRequestMsg{prompt: prompt}
		case <-ctx.Done():
			return nil
		}
	}
}

// sendConfirm отправляет ответ пользователя инструменту и снова ждёт следующий запрос.
func sendConfirm(ctx context.Context, reqCh chan string, respCh chan bool, ok bool) tea.Cmd {
	return func() tea.Msg {
		select {
		case respCh <- ok:
		case <-ctx.Done():
			return nil
		}
		select {
		case prompt := <-reqCh:
			return confirmRequestMsg{prompt: prompt}
		case <-ctx.Done():
			return nil
		}
	}
}

// View рисует кадр интерфейса: приветствие, ленту диалога, статус и поле ввода.
func (m tuiModel) View() string {
	var b strings.Builder

	welcome := titleStyle.Render("✻ Mini Code") + "\n" +
		dimStyle.Render("агент-кодер на Go · знает ваш проект · exit — выход")
	b.WriteString(boxStyle.Render(welcome))
	b.WriteString("\n\n")

	for _, line := range m.history {
		b.WriteString(line)
		b.WriteByte('\n')
	}

	switch m.state {
	case stThinking:
		b.WriteString(m.spin.View() + dimStyle.Render(" думаю..."))
		b.WriteByte('\n')
	case stConfirm:
		b.WriteString(confirmStyle.Render("ожидаю ответ: y (да) / n (нет)"))
		b.WriteByte('\n')
	}

	b.WriteString(inputStyle.Render(m.input.View()))
	return b.String()
}

// runTUI запускает терминальный интерфейс. confirm подменяется на канальную
// версию: в TUI стандартный ввод занят интерфейсом, поэтому подтверждения
// инструментов идут через каналы модели, а не через bufio.Reader.
func runTUI(parent context.Context, ag *react.Agent) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel() // при выходе отменяем фоновые команды — без утечек горутин

	reqCh := make(chan string)
	respCh := make(chan bool)
	confirm = func(prompt string) bool {
		reqCh <- prompt
		return <-respCh
	}

	p := tea.NewProgram(newTUIModel(ctx, ag, reqCh, respCh))
	_, err := p.Run()
	return err
}
