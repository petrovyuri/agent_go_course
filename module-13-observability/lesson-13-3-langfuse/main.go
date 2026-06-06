// module-13-observability/lesson-13-3-langfuse/main.go

// Урок 13.3. Трейсинг агента через LangFuse.
//
// Callbacks из урока 13.1 печатали трассу в консоль. Для настоящей отладки удобнее
// присылать трейсы в LangFuse — веб-интерфейс, где видно дерево вызовов агента,
// время, токены и содержимое каждого шага (наш аналог LangGraph Studio).
//
// Готовый обработчик callbacks для LangFuse есть в eino-ext: langfuse.NewLangfuseHandler.
// Вешаем его глобально через callbacks.AppendGlobalHandlers — и каждый запуск агента
// автоматически уходит в LangFuse. Ключи берём из переменных окружения (не хардкодим
// и не храним в коде!).
//
// Подготовка (один раз):
//   - LangFuse Cloud (бесплатный тариф): https://cloud.langfuse.com — создайте
//     проект, скопируйте Public Key (pk-lf-...) и Secret Key (sk-lf-...);
//   - либо self-host через Docker: https://langfuse.com/self-hosting.
//   - задайте ключи одним из двух способов:
//     1) переменные окружения LANGFUSE_BASE_URL / LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY;
//     2) файл .env рядом с программой (строки KEY=VALUE) — она подхватит его сама.
//     Файл .env добавьте в .gitignore, чтобы не закоммитить секреты!
//
// Запуск из папки урока:
//
//	go mod tidy
//	go run .
//
// Нужен запущенный Ollama с моделью qwen3.5 и заданные ключи LangFuse.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"      // обработчик трейсинга в LangFuse
	"github.com/cloudwego/eino-ext/components/model/ollama" // ChatModel для Ollama
	"github.com/cloudwego/eino/callbacks"                   // AppendGlobalHandlers
	"github.com/cloudwego/eino/components/tool"             // BaseTool
	"github.com/cloudwego/eino/components/tool/utils"       // конструкторы инструментов
	"github.com/cloudwego/eino/compose"                     // ToolsNodeConfig
	"github.com/cloudwego/eino/flow/agent/react"            // готовый ReAct-агент
	"github.com/cloudwego/eino/schema"                      // Message
)

const (
	ollamaBaseURL = "http://localhost:11434" // адрес локального сервера Ollama
	modelName     = "qwen3.5"                // модель с поддержкой инструментов
	maxStep       = 8                        // лимит шагов ReAct
)

// addArgs — параметры инструмента add.
type addArgs struct {
	A int `json:"a" jsonschema:"required" jsonschema_description:"первое слагаемое"`
	B int `json:"b" jsonschema:"required" jsonschema_description:"второе слагаемое"`
}

// add складывает два целых числа.
func add(_ context.Context, in addArgs) (int, error) {
	return in.A + in.B, nil
}

// buildAgent собирает простого ReAct-агента с инструментом add (как в уроке 13.1).
func buildAgent(ctx context.Context) (*react.Agent, error) {
	addTool, err := utils.InferTool("add", "Складывает два целых числа", add)
	if err != nil {
		return nil, fmt.Errorf("инструмент add: %w", err)
	}
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  ollamaBaseURL,
		Model:    modelName,
		Thinking: &ollama.ThinkValue{Value: false},
	})
	if err != nil {
		return nil, fmt.Errorf("создание ChatModel: %w", err)
	}
	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: []tool.BaseTool{addTool}},
		MaxStep:          maxStep,
	})
}

// loadEnvFile подхватывает переменные из файла .env рядом с программой (строки
// вида KEY=VALUE), если он есть. Go сам .env НЕ читает — os.Getenv смотрит только
// в окружение процесса, поэтому файл надо загрузить явно. Уже заданные в окружении
// переменные в приоритете — их не перезаписываем. Кавычки и пробелы вокруг = снимаем.
func loadEnvFile() {
	f, err := os.Open(".env")
	if err != nil {
		return // нет .env — работаем с тем, что уже в окружении
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // пустые строки и комментарии пропускаем
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimPrefix(strings.TrimSpace(key), "export ") // допускаем строки вида export KEY=...
		val = strings.Trim(strings.TrimSpace(val), "\"'")
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}

// langfuseAuthStatus проверяет адрес и ключи: дергает /api/public/projects с
// Basic-авторизацией и возвращает понятный диагноз. Это важно, потому что при
// неверных ключах или не том регионе LangFuse молча отбрасывает трейс (SDK глотает
// ответы 4xx) — кажется, что всё ушло, а в проекте пусто. Тут мы видим причину.
func langfuseAuthStatus(ctx context.Context, host, pub, sec string) string {
	url := strings.TrimRight(host, "/") + "/api/public/projects"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // адрес LangFuse задаёт владелец через env — это его сервер
	if err != nil {
		return "проверка LangFuse не удалась: " + err.Error()
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(pub+":"+sec)))
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req) //nolint:gosec // адрес LangFuse задаёт владелец через env — это его сервер
	if err != nil {
		return "LangFuse недоступен по адресу " + host + ": " + err.Error() + " (проверьте адрес и сеть/прокси)"
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return "LangFuse: адрес и ключи верны — трейсы появятся в проекте."
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("LangFuse вернул %d: неверные ключи ИЛИ не тот регион. "+
			"EU = https://cloud.langfuse.com, US = https://us.cloud.langfuse.com (ключи привязаны к региону).", resp.StatusCode)
	default:
		return fmt.Sprintf("LangFuse вернул статус %d по адресу %s.", resp.StatusCode, host)
	}
}

func main() {
	ctx := context.Background()

	// Подхватываем .env (если есть), затем читаем ключи из окружения.
	loadEnvFile()
	host := os.Getenv("LANGFUSE_BASE_URL")
	pub := os.Getenv("LANGFUSE_PUBLIC_KEY")
	sec := os.Getenv("LANGFUSE_SECRET_KEY")
	if host == "" || pub == "" || sec == "" {
		log.Println("Не заданы LANGFUSE_BASE_URL / LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY.")
		log.Println("Задайте их в окружении или в файле .env рядом (строки KEY=VALUE).")
		log.Println("Ключи берут в LangFuse Cloud (https://cloud.langfuse.com, бесплатный тариф) или в self-host.")
		return
	}

	// Сразу проверяем связь и ключи — иначе трейс молча уйдёт в никуда.
	log.Print(langfuseAuthStatus(ctx, host, pub, sec)) //nolint:gosec // диагностика подключения: адрес из конфигурации LangFuse

	// Глобальный обработчик: трейсы каждого запуска уходят в LangFuse.
	handler, flusher := langfuse.NewLangfuseHandler(&langfuse.Config{
		Host:      host,
		PublicKey: pub,
		SecretKey: sec,
	})
	defer flusher() // дослать накопленные трейсы перед выходом
	callbacks.AppendGlobalHandlers(handler)

	agentRunner, err := buildAgent(ctx)
	if err != nil {
		log.Printf("не удалось собрать агента: %v", err)
		return // не Fatalf: дать отложенному flusher() отработать
	}

	msgs := []*schema.Message{
		schema.SystemMessage("Ты считаешь только через инструмент add. Отвечай на русском, кратко."),
		schema.UserMessage("Сколько будет 7 + 5, а потом прибавь к результату 100?"),
	}
	answer, err := agentRunner.Generate(ctx, msgs)
	if err != nil {
		log.Printf("ошибка агента: %v", err)
		return // не Fatalf: дать отложенному flusher() отработать
	}

	fmt.Println(answer.Content)
	fmt.Println("Трейс отправлен в LangFuse — откройте проект и посмотрите дерево вызовов.")
}
