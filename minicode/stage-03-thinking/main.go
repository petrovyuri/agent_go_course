// minicode/stage-03-thinking/main.go

// Mini Code — мини-агент-кодер на Go (Eino + Ollama). Этап 3: думающий агент.
// Раньше Mini Code делал один раунд инструментов на запрос. Теперь он крутит
// настоящий ReAct-цикл через react.Agent: осматривается, ищет, читает файлы
// несколько шагов подряд, пока не соберёт ответ. Инструменты те же из модуля 6
// (read_file, list_dir, grep, см. tools.go). Памяти между запросами пока нет —
// это тема модуля 9, а правка файлов с подтверждением будет в модуле 10.
//
// Запуск из папки модуля:
//
//	go mod tidy
//	go run .
package main

import (
	"bufio"   // построчное чтение из терминала
	"context" // контекст: таймаут и отмена
	"fmt"     // ввод-вывод в консоль
	"log"     // log.Fatalf/Printf
	"os"      // доступ к stdin
	"strings" // обрезка пробелов во вводе

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
	maxStep       = 16                       // лимит шагов ReAct (защита от зацикливания)
	systemPrompt  = "Ты Mini Code — помощник-кодер. Для работы с файлами вызывай " +
		"инструменты read_file, list_dir и grep. Действуй по шагам: если не знаешь, где " +
		"искать, сначала осмотрись через list_dir или найди место через grep, потом " +
		"прочитай нужный файл. Отвечай на русском, кратко и по делу."
)

// buildAgent создаёт инструменты чтения и собирает ReAct-агента на их основе.
func buildAgent(ctx context.Context) (*react.Agent, error) {
	// Три инструмента на чтение. Функции и их параметры — в tools.go.
	readTool, err := utils.InferTool("read_file", "Читает текстовый файл по пути", readFile)
	if err != nil {
		return nil, fmt.Errorf("инструмент read_file: %w", err)
	}
	listTool, err := utils.InferTool("list_dir", "Показывает содержимое папки", listDir)
	if err != nil {
		return nil, fmt.Errorf("инструмент list_dir: %w", err)
	}
	grepTool, err := utils.InferTool("grep", "Ищет подстроку в файле или во всех файлах папки (рекурсивно)", grep)
	if err != nil {
		return nil, fmt.Errorf("инструмент grep: %w", err)
	}

	// Модель. Размышления выключаем — без них ответы быстрее (как в модуле 6).
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}

	// ReAct-агент: модель + инструменты + лимит шагов. Цикл агент крутит сам.
	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{readTool, listTool, grepTool}},
		MaxStep:          maxStep,
	})
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

	fmt.Println("Mini Code готов. Напишите запрос (exit — выход).")

	// REPL: каждый запрос — отдельный запуск ReAct-агента (память между запросами не ведём).
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break // конец ввода (Ctrl+D)
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" {
			break
		}

		// Системную инструкцию даём первым сообщением, дальше — запрос пользователя.
		answer, err := agent.Generate(ctx, []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(input),
		})
		if err != nil {
			fmt.Println("ошибка:", err)
			continue
		}
		fmt.Println(answer.Content)
	}

	fmt.Println("Пока!")
}
