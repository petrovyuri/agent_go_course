// minicode/stage-06-final/index.go

// Знание проекта через RAG. При старте Mini Code индексирует свою кодовую базу:
// режет файлы на куски, превращает каждый кусок в вектор эмбеддером Ollama и
// складывает в память (это тот же приём, что memStore из модуля 11). Инструмент
// search_code потом ищет по смыслу куски, похожие на запрос, — семантический
// поиск в дополнение к буквальному grep.
package main

import (
	"context"       // контекст для эмбеддера
	"fmt"           // сборка строк
	"go/ast"        // узлы синтаксического дерева Go
	"go/parser"     // разбор Go-файла на объявления
	"go/token"      // позиции в исходнике
	"io/fs"         // обход дерева файлов
	"math"          // косинусная близость
	"os"            // чтение файлов
	"path/filepath" // работа с путями
	"sort"          // сортировка результатов по близости
	"strings"       // разбиение на куски

	"github.com/cloudwego/eino-ext/components/embedding/ollama" // эмбеддер Ollama
	"github.com/cloudwego/eino/components/embedding"            // интерфейс Embedder
)

const (
	embedModelName = "nomic-embed-text" // эмбеддинг-модель, заточенная под семантический поиск
	chunkLines     = 40                 // строк в куске для не-Go файлов
	indexTopK      = 4                  // сколько кусков возвращает поиск
	embedBatch     = 32                 // сколько кусков эмбеддим за один вызов
	maxChunks      = 400                // потолок: не индексируем бесконечно

	// nomic-embed-text различает документы и запросы по префиксу задачи —
	// без них качество поиска заметно падает.
	docPrefix   = "search_document: "
	queryPrefix = "search_query: "
)

// codeExts — какие файлы индексируем (исходники и заметки проекта).
var codeExts = map[string]bool{".go": true, ".md": true}

// chunk — кусок файла вместе с его вектором.
type chunk struct {
	path string
	line int // номер первой строки куска (с 1)
	text string
	vec  []float64
}

// projectIndex — векторный индекс кодовой базы в памяти. Внутри то же, что
// memStore из модуля 11: эмбеддер, список кусков с векторами и косинусный поиск.
type projectIndex struct {
	embedder embedding.Embedder
	chunks   []chunk
}

// newProjectIndex создаёт эмбеддер Ollama и пустой индекс.
func newProjectIndex(ctx context.Context, baseURL string) (*projectIndex, error) {
	emb, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{BaseURL: baseURL, Model: embedModelName})
	if err != nil {
		return nil, fmt.Errorf("создание эмбеддера: %w", err)
	}
	return &projectIndex{embedder: emb}, nil
}

// chunkFile режет файл на куски. Go-файлы — по объявлениям (функция/тип = один
// осмысленный кусок), остальное — окнами по строкам.
func chunkFile(path, content string) []chunk {
	if strings.HasSuffix(path, ".go") {
		if c := chunkGoSource(path, content); c != nil {
			return c
		}
	}
	return splitChunks(path, content)
}

// chunkGoSource режет Go-файл на куски по верхнеуровневым объявлениям (func,
// type, const, var) вместе с их doc-комментариями. Так каждый кусок — один
// смысловой символ, а не случайное окно строк. Если файл не разобрался, вернёт
// nil (тогда упадём на построчную нарезку).
func chunkGoSource(path, content string) []chunk {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil
	}
	var out []chunk
	for _, d := range f.Decls {
		// Индексируем логику — функции и типы. const/var/import — это конфиг,
		// он засоряет поиск, поэтому пропускаем.
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok != token.TYPE {
			continue
		}
		startOff := fset.Position(d.Pos()).Offset
		if doc := declDoc(d); doc != nil {
			startOff = fset.Position(doc.Pos()).Offset // прихватываем doc-комментарий
		}
		endOff := fset.Position(d.End()).Offset
		text := strings.TrimSpace(content[startOff:endOff])
		if text == "" {
			continue
		}
		out = append(out, chunk{path: path, line: fset.Position(d.Pos()).Line, text: text})
	}
	return out
}

// declDoc возвращает doc-комментарий объявления (если есть).
func declDoc(d ast.Decl) *ast.CommentGroup {
	switch t := d.(type) {
	case *ast.FuncDecl:
		return t.Doc
	case *ast.GenDecl:
		return t.Doc
	}
	return nil
}

// splitChunks режет текст на куски по chunkLines строк (для не-Go файлов).
func splitChunks(path, content string) []chunk {
	lines := strings.Split(content, "\n")
	var out []chunk
	for start := 0; start < len(lines); start += chunkLines {
		end := min(start+chunkLines, len(lines))
		text := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
		if text == "" {
			continue
		}
		out = append(out, chunk{path: path, line: start + 1, text: text})
	}
	return out
}

// collectChunks обходит проект и собирает куски из подходящих файлов (пропускает
// скрытые папки, vendor/node_modules, .env и слишком большие файлы).
func collectChunks(root string) ([]chunk, error) {
	var chunks []chunk
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, ".env") || strings.HasSuffix(lower, "_test.go") || !codeExts[filepath.Ext(lower)] {
			return nil // секреты, тесты и не-код пропускаем
		}
		data, errRead := os.ReadFile(p) //nolint:gosec // индексируем файлы проекта; путь приходит из WalkDir по рабочей папке
		if errRead != nil || len(data) > maxFileSize {
			return nil //nolint:nilerr // нечитаемый или огромный файл просто пропускаем
		}
		chunks = append(chunks, chunkFile(filepath.ToSlash(p), string(data))...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("обход проекта: %w", err)
	}
	if len(chunks) > maxChunks {
		chunks = chunks[:maxChunks]
	}
	return chunks, nil
}

// indexProject индексирует кодовую базу из папки root: собирает куски, эмбеддит
// их батчами и сохраняет векторы. Возвращает число проиндексированных кусков.
func (idx *projectIndex) indexProject(ctx context.Context, root string) (int, error) {
	chunks, err := collectChunks(root)
	if err != nil {
		return 0, err
	}
	for start := 0; start < len(chunks); start += embedBatch {
		end := min(start+embedBatch, len(chunks))
		texts := make([]string, end-start)
		for i := range texts {
			texts[i] = docPrefix + chunks[start+i].text
		}
		vectors, errEmb := idx.embedder.EmbedStrings(ctx, texts)
		if errEmb != nil {
			return 0, fmt.Errorf("эмбеддинг кусков: %w", errEmb)
		}
		for i := range vectors {
			chunks[start+i].vec = vectors[i]
		}
	}
	idx.chunks = chunks
	return len(chunks), nil
}

// search возвращает k кусков, ближайших к запросу по косинусной близости.
func (idx *projectIndex) search(ctx context.Context, query string, k int) ([]chunk, error) {
	if len(idx.chunks) == 0 {
		return nil, nil
	}
	qv, err := idx.embedder.EmbedStrings(ctx, []string{queryPrefix + query})
	if err != nil {
		return nil, fmt.Errorf("эмбеддинг запроса: %w", err)
	}
	ranked := make([]chunk, len(idx.chunks))
	copy(ranked, idx.chunks)
	sort.Slice(ranked, func(i, j int) bool {
		return cosine(qv[0], ranked[i].vec) > cosine(qv[0], ranked[j].vec)
	})
	return ranked[:min(k, len(ranked))], nil
}

// cosine — косинусная близость двух векторов (1.0 — похожи, 0 — нет).
func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
