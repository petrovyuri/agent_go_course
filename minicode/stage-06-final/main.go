// minicode/stage-06-final/main.go

// Mini Code — мини-агент-кодер на Go (Eino + Ollama). Этап 6: финал.
// Берём агента из этапа 5 (RAG по проекту + MCP) и доводим его до готового
// инструмента:
//   - наблюдаемость: при наличии ключей трейсы запусков уходят в LangFuse
//     (Callbacks из модуля 13) — видно дерево вызовов, время и токены;
//   - конфигурация: флаги командной строки (-model, -q, -trace);
//   - UX: одноразовый режим -q "вопрос" (ответил и вышел) для скриптов и CI,
//     интерактивный REPL — для работы руками.
//
// Запуск из папки модуля:
//
//	go mod tidy
//	go run .                          # интерактивный режим (REPL)
//	go run . -q "что делает safePath" # одноразовый вопрос и выход
//	go run . -model qwen2.5 -trace=false
package main

import (
	"bufio"         // чтение из терминала
	"context"       // контекст и таймауты
	"encoding/json" // сохранение/загрузка сессии
	"flag"          // флаги командной строки
	"fmt"           // ввод-вывод в консоль
	"log"           // log.Printf
	"os"            // stdin, файлы, переменные окружения
	"strings"       // обработка ввода и .env
	"time"          // таймаут хода

	"github.com/cloudwego/eino-ext/callbacks/langfuse"      // трейсинг в LangFuse
	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/callbacks"                   // AppendGlobalHandlers
	"github.com/cloudwego/eino/components/tool"             // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNodeConfig
	"github.com/cloudwego/eino/flow/agent/react"            // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                      // Message
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	sessionFile   = "session.json"           // файл памяти сессии
	turnTimeout   = 5 * time.Minute          // таймаут на один ход агента
	maxStep       = 16                       // лимит шагов ReAct (защита от зацикливания)
	systemPrompt  = "Ты Mini Code — помощник-кодер, который знает этот проект. " +
		"Поиск по коду: search_code (по смыслу) и grep (буквальный). Чтение: read_file, list_dir. " +
		"Правка: write_file, edit_file. Команды: run_command (только go). Внешние инструменты MCP: " +
		"git_log (история коммитов), project_tree (дерево проекта). Если вопрос про проект — сначала " +
		"найди нужный код через search_code, потом отвечай по нему. Действуй по шагам; перед изменением " +
		"коротко скажи, что собираешься сделать. Отвечай на русском, кратко."
)

// config — настройки запуска из флагов командной строки.
type config struct {
	model string // модель Ollama
	query string // одноразовый вопрос (пусто — интерактивный REPL)
	trace bool   // слать трейсы в LangFuse (если заданы ключи)
}

// parseConfig читает флаги командной строки.
func parseConfig() config {
	var c config
	flag.StringVar(&c.model, "model", "qwen3.5", "модель Ollama для агента")
	flag.StringVar(&c.query, "q", "", "одноразовый вопрос: выполнить и выйти (без REPL)")
	flag.BoolVar(&c.trace, "trace", true, "слать трейсы в LangFuse, если заданы ключи окружения")
	flag.Parse()
	return c
}

// reader — общий ввод из терминала: им пользуется и REPL, и подтверждение в
// инструментах, поэтому буфер ввода один на всех.
var reader = bufio.NewReader(os.Stdin)

// readLine читает одну строку из общего ввода (без хвостовых пробелов).
func readLine() (string, bool) {
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", false // конец ввода
	}
	return strings.TrimSpace(line), true
}

// confirm спрашивает подтверждение перед опасным действием. Это переменная —
// так её удобно подменить в тестах.
var confirm = func(prompt string) bool {
	fmt.Print("Mini Code просит подтверждение — " + prompt + " — выполнить? (y/n): ")
	line, ok := readLine()
	if !ok {
		return false
	}
	ans := strings.ToLower(line)
	return ans == "y" || ans == "yes" || ans == "да"
}

// loadEnvFile подхватывает переменные из файла .env рядом с программой (строки
// KEY=VALUE), если он есть. Go сам .env не читает. Уже заданные переменные не трогаем.
func loadEnvFile() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimPrefix(strings.TrimSpace(key), "export ")
		val = strings.Trim(strings.TrimSpace(val), "\"'")
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}

// setupTracing включает трейсинг в LangFuse, если это не отключено флагом и заданы
// ключи (LANGFUSE_BASE_URL/PUBLIC_KEY/SECRET_KEY, см. модуль 13). Возвращает
// flusher — его нужно вызвать перед выходом, чтобы дослать накопленные трейсы.
func setupTracing(enabled bool) func() {
	noop := func() {}
	if !enabled {
		return noop
	}
	loadEnvFile()
	host := os.Getenv("LANGFUSE_BASE_URL")
	pub := os.Getenv("LANGFUSE_PUBLIC_KEY")
	sec := os.Getenv("LANGFUSE_SECRET_KEY")
	if host == "" || pub == "" || sec == "" {
		log.Println("Трейсинг выключен: нет ключей LangFuse (LANGFUSE_BASE_URL/PUBLIC_KEY/SECRET_KEY).")
		return noop
	}
	handler, flusher := langfuse.NewLangfuseHandler(&langfuse.Config{Host: host, PublicKey: pub, SecretKey: sec})
	callbacks.AppendGlobalHandlers(handler)
	log.Println("Трейсинг включён: запуски уходят в LangFuse.")
	return flusher
}

// buildAgent собирает ReAct-агента: чтение, семантический поиск, правка, команды
// и инструменты внешнего MCP-сервера (extraTools). Модель берётся из конфига.
func buildAgent(ctx context.Context, idx *projectIndex, model string, extraTools []tool.BaseTool) (*react.Agent, error) {
	// Модель. Размышления выключаем — без них ответы быстрее (как в модуле 8).
	// Её же передаём в search_code для переписывания запроса (agentic RAG, см. agentic.go).
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    model,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	readTool, err := utils.InferTool("read_file", "Читает текстовый файл по пути", readFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент read_file: %w", err)
	}
	listTool, err := utils.InferTool("list_dir", "Показывает содержимое папки", listDir)
	if err != nil {
		return nil, fmt.Errorf("инструмент list_dir: %w", err)
	}
	grepTool, err := utils.InferTool("grep", "Ищет подстроку в файле или во всех файлах папки", grep)
	if err != nil {
		return nil, fmt.Errorf("инструмент grep: %w", err)
	}
	searchTool, err := utils.InferTool("search_code", "Семантический поиск по коду проекта: ищет куски, похожие по смыслу", makeSearchCode(idx, chatModel))
	if err != nil {
		return nil, fmt.Errorf("инструмент search_code: %w", err)
	}
	writeTool, err := utils.InferTool("write_file", "Создаёт или перезаписывает файл (с подтверждением)", writeFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент write_file: %w", err)
	}
	editTool, err := utils.InferTool("edit_file", "Заменяет подстроку в файле (с подтверждением)", editFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент edit_file: %w", err)
	}
	cmdTool, err := utils.InferTool("run_command", "Запускает команду go (с подтверждением)", runCommand)
	if err != nil {
		return nil, fmt.Errorf("инструмент run_command: %w", err)
	}

	tools := []tool.BaseTool{readTool, listTool, grepTool, searchTool, writeTool, editTool, cmdTool}
	tools = append(tools, extraTools...)
	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
		MaxStep:          maxStep,
	})
}

// loadSession читает историю сессии с диска или начинает новую с системным промптом.
func loadSession() []*schema.Message {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return []*schema.Message{schema.SystemMessage(systemPrompt)}
	}
	var msgs []*schema.Message
	if err := json.Unmarshal(data, &msgs); err != nil || len(msgs) == 0 {
		return []*schema.Message{schema.SystemMessage(systemPrompt)}
	}
	return msgs
}

// saveSession сохраняет историю сессии на диск.
func saveSession(msgs []*schema.Message) {
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		fmt.Println("не удалось сериализовать сессию:", err)
		return
	}
	if err := os.WriteFile(sessionFile, data, 0o600); err != nil {
		fmt.Println("не удалось сохранить сессию:", err)
	}
}

// repl — интерактивный режим: на каждый запрос прогоняем агента по всей истории.
func repl(ctx context.Context, agent *react.Agent) {
	msgs := loadSession()
	fmt.Println("Mini Code готов и знает проект. Помнит сессию между запусками.")
	fmt.Println("Команды: exit — выход (сессия сохранится), /reset — забыть историю.")
	for {
		fmt.Print("> ")
		input, ok := readLine()
		if !ok {
			break // конец ввода (Ctrl+D)
		}
		switch input {
		case "":
			continue
		case "exit":
			saveSession(msgs)
			fmt.Println("Пока! Сессия сохранена.")
			return
		case "/reset":
			msgs = []*schema.Message{schema.SystemMessage(systemPrompt)}
			saveSession(msgs)
			fmt.Println("История очищена.")
			continue
		}

		msgs = append(msgs, schema.UserMessage(input))
		genCtx, cancel := context.WithTimeout(ctx, turnTimeout)
		answer, err := agent.Generate(genCtx, msgs)
		cancel()
		if err != nil {
			fmt.Println("ошибка:", err)
			continue
		}
		msgs = append(msgs, answer)
		fmt.Println(answer.Content)
		saveSession(msgs)
	}
}

func main() {
	cfg := parseConfig()
	ctx := context.Background()

	// Наблюдаемость: трейсы запусков уходят в LangFuse (если заданы ключи).
	flusher := setupTracing(cfg.trace)
	defer flusher()

	// Знание проекта: индексируем кодовую базу текущей папки.
	idx, err := newProjectIndex(ctx, ollamaBaseURL)
	if err != nil {
		log.Printf("не удалось создать индекс: %v", err)
		return
	}
	fmt.Println("Индексирую проект...")
	n, err := idx.indexProject(ctx, ".")
	if err != nil {
		log.Printf("не удалось проиндексировать проект: %v", err)
		return
	}
	fmt.Printf("Готово: проиндексировано кусков кода — %d.\n", n)

	// Внешний MCP-сервер (git_log, project_tree). Не вышло — работаем без него.
	mcpTools, mcpCli, err := connectProjectMCP(ctx)
	if err != nil {
		log.Printf("MCP-сервер не подключён: %v", err)
	} else {
		defer mcpCli.Close()
		fmt.Printf("MCP подключён: получено инструментов — %d.\n", len(mcpTools))
	}

	agent, err := buildAgent(ctx, idx, cfg.model, mcpTools)
	if err != nil {
		log.Printf("не удалось собрать агента: %v", err)
		return
	}

	// Одноразовый режим: ответил на вопрос из -q и вышел (удобно для скриптов и CI).
	if cfg.query != "" {
		msgs := []*schema.Message{schema.SystemMessage(systemPrompt), schema.UserMessage(cfg.query)}
		genCtx, cancel := context.WithTimeout(ctx, turnTimeout)
		answer, err := agent.Generate(genCtx, msgs)
		cancel()
		if err != nil {
			log.Printf("ошибка агента: %v", err)
			return
		}
		fmt.Println(answer.Content)
		return
	}

	// Интерактивный режим: поднимаем EinoDev (граф виден, пока работает REPL) и крутим REPL.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}
	repl(ctx, agent)
}
