# Создание AI-агентов на Go с нуля: Eino, ReAct, RAG, MCP — код курса

Здесь лежит рабочий код к урокам курса. Агенты строятся на фреймворке
[Eino](https://github.com/cloudwego/eino) от CloudWeGo, модели запускаются локально через
[Ollama](https://ollama.com) — без облачных ключей и оплаты за токены.

## Как устроено

Код сгруппирован по модулям и урокам. **Каждый урок — самостоятельный Go-модуль** со своим
`go.mod`, поэтому уроки можно запускать независимо друг от друга.

```
course/
└── module-01-introduction/
    ├── lesson-04-setup/          # проверка окружения (чистый Go, без Eino)
    │   ├── go.mod
    │   ├── main.go
    │   └── main_test.go
    └── lesson-05-first-run/      # первый вызов модели через Eino
        ├── go.mod
        └── main.go
```

## Подготовка

```bash
# Go 1.26+  -> https://go.dev/dl/
go version

# Ollama    -> https://ollama.com/download
ollama --version

# модель с поддержкой инструментов (tool calling)
ollama pull qwen3.5
```

## Запуск урока

Перейдите в папку нужного урока и запустите:

```bash
cd module-01-introduction/lesson-05-first-run
go mod tidy   # один раз: подтянет зависимости
go run .
```

Урок 1.4 (проверка окружения) зависимостей не требует — там достаточно `go run .`.

## Тесты

В уроках, где это уместно, есть тесты. Запуск из папки урока:

```bash
go test ./...
```
