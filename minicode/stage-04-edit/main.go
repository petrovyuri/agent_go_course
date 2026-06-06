// minicode/stage-04-edit/main.go

// Mini Code — мини-агент-кодер на Go (Eino + Ollama). Этап 4: память, правка,
// подтверждения. Берём думающего агента из модуля 8 (react.Agent на графе) и
// добавляем к нему:
//   - память сессии: историю диалога храним и сохраняем на диск (session.json),
//     поэтому Mini Code помнит разговор между запусками;
//   - право менять код: инструменты write_file, edit_file и run_command (только go);
//   - подтверждение (HITL): опасные инструменты сами спрашивают человека перед
//     действием (см. confirm) — ничего не меняется и не запускается без вашего "да";
//   - надёжность: на каждый ход агента ставим таймаут через контекст.
//
// Граф агента по-прежнему виден в EinoDev (devops.Init).
//
// Запуск из папки модуля:
//
//	go mod tidy
//	go run .
package main

import (
	"bufio"         // чтение из терминала
	"context"       // контекст и таймауты
	"encoding/json" // сохранение/загрузка сессии
	"fmt"           // ввод-вывод в консоль
	"log"           // log.Fatalf/Printf
	"os"            // stdin и файл сессии
	"strings"       // обработка ввода
	"time"          // таймаут хода

	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino-ext/devops"                  // сервер EinoDev для просмотра графа
	"github.com/cloudwego/eino/components/tool"             // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNodeConfig
	"github.com/cloudwego/eino/flow/agent/react"            // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                      // Message
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	sessionFile   = "session.json"           // файл памяти сессии
	turnTimeout   = 5 * time.Minute          // таймаут на один ход агента
	maxStep       = 16                       // лимит шагов ReAct (защита от зацикливания)
	systemPrompt  = "Ты Mini Code — помощник-кодер, который умеет читать и менять код. " +
		"Инструменты чтения: read_file, list_dir, grep. Инструменты правки: write_file, " +
		"edit_file. Команды: run_command (только go). Действуй по шагам; перед изменением " +
		"коротко скажи, что собираешься сделать. Отвечай на русском, кратко."
)

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

// buildAgent собирает ReAct-агента со всеми инструментами (чтение + правка + команды).
func buildAgent(ctx context.Context) (*react.Agent, error) {
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

	// Модель. Размышления выключаем — без них ответы быстрее (как в модуле 8).
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	// ReAct-агент с шестью инструментами. Цикл агент крутит сам.
	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{Tools: []tool.BaseTool{
			readTool, listTool, grepTool, writeTool, editTool, cmdTool,
		}},
		MaxStep: maxStep,
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

func main() {
	ctx := context.Background()

	// Поднимаем сервер EinoDev — граф ReAct-агента виден в плагине, пока работает REPL.
	if err := devops.Init(ctx); err != nil {
		log.Printf("EinoDev не запущен: %v", err)
	}

	agent, err := buildAgent(ctx)
	if err != nil {
		log.Fatalf("не удалось собрать агента: %v", err)
	}

	msgs := loadSession()
	fmt.Println("Mini Code готов. Помнит сессию между запусками.")
	fmt.Println("Команды: exit — выход (сессия сохранится), /reset — забыть историю.")

	// REPL: на каждый запрос прогоняем агента по всей истории (это и есть память).
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

		// Таймаут на ход: если модель или команда зависнут, ход прервётся.
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
