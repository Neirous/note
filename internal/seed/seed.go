package seed

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	_ "modernc.org/sqlite"

	"note/internal/llm"
	"note/internal/rag"
	"note/internal/store"
)

type SeedDoc struct {
	Key       string
	Title     string
	FolderKey string
	Tags      []string
	Markdown  string
}

type Indexer struct {
	store    *store.Store
	md       goldmark.Markdown
	embedder rag.EmbeddingProvider
}

func NewIndexer(st *store.Store, embedder rag.EmbeddingProvider) *Indexer {
	return &Indexer{
		store:    st,
		md:       goldmark.New(),
		embedder: embedder,
	}
}

func (idx *Indexer) IndexNote(ctx context.Context, noteID int64, markdown string) error {
	ragSvc := rag.NewService(idx.store, idx.embedder, nil, rag.Config{MaxChunkChars: 800, TopK: 5})
	return ragSvc.IndexNote(ctx, noteID, markdown)
}

func Run(ctx context.Context, dsn string) error {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	st := store.New(db)
	if err := st.InitSchema(ctx); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	var embedder rag.EmbeddingProvider
	if apiKey != "" {
		baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
		embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
		chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")
		embedder = llm.NewOpenAICompatibleClient(&http.Client{Timeout: 90 * time.Second}, baseURL, apiKey, embedModel, chatModel)
	}

	idx := NewIndexer(st, embedder)

	fmt.Println("==> Creating folders and demo notes...")
	created, err := upsertDocs(ctx, st, idx, apiKey != "")
	if err != nil {
		return err
	}
	fmt.Printf("==> Done: %d notes created/updated\n", len(created))
	return nil
}

func upsertDocs(ctx context.Context, st *store.Store, idx *Indexer, hasAPIKey bool) ([]SeedDoc, error) {
	folders := defaultFolders()
	pages := defaultPages()

	folderByKey := map[string]*store.Note{}
	for _, f := range folders {
		note, err := upsertNote(ctx, st, f.Title, f.Tags, nil, f.Markdown)
		if err != nil {
			return nil, err
		}
		folderByKey[f.Key] = note
	}

	all := append(folders, pages...)
	for _, doc := range pages {
		var parentID *int64
		if doc.FolderKey != "" {
			if folder, ok := folderByKey[doc.FolderKey]; ok {
				parentID = &folder.ID
			}
		}
		note, err := upsertNote(ctx, st, doc.Title, doc.Tags, parentID, doc.Markdown)
		if err != nil {
			return nil, err
		}
		_ = note
		if hasAPIKey {
			if err := idx.IndexNote(ctx, note.ID, note.Markdown); err != nil {
				fmt.Printf("  warning: index %q: %v\n", doc.Title, err)
			}
		}
	}

	return all, nil
}

func upsertNote(ctx context.Context, st *store.Store, title string, tags []string, parentID *int64, markdown string) (*store.Note, error) {
	existing, err := st.ListNotes(ctx, store.NoteFilter{Query: title})
	if err != nil {
		return nil, err
	}
	for _, n := range existing {
		if strings.EqualFold(strings.TrimSpace(n.Title), strings.TrimSpace(title)) {
			note, err := st.UpdateNote(ctx, n.ID, store.NoteInput{
				ParentID: parentID, Title: title, Markdown: markdown, HTML: renderMarkdown(markdown), Tags: tags,
			})
			if err != nil {
				return nil, err
			}
			return &note, nil
		}
	}
	note, err := st.CreateNote(ctx, store.NoteInput{
		ParentID: parentID, Title: title, Markdown: markdown, HTML: renderMarkdown(markdown), Tags: tags,
	})
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func renderMarkdown(md string) string {
	var buf bytes.Buffer
	if err := goldmark.New().Convert([]byte(md), &buf); err != nil {
		return md
	}
	return buf.String()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultFolders() []SeedDoc {
	return []SeedDoc{
		{Key: "backend_folder", Title: "后端工程", Tags: []string{"folder", "catalog"}, Markdown: "# 后端工程\n\n这个目录收纳 Go、HTTP、数据库与检索相关页面，适合演示分层管理与页面跳转。"},
		{Key: "ai_folder", Title: "AI 与检索", Tags: []string{"folder", "catalog"}, Markdown: "# AI与检索\n\n这个目录收纳 RAG、向量化与全文检索相关页面。"},
		{Key: "frontend_folder", Title: "前端体验", Tags: []string{"folder", "catalog"}, Markdown: "# 前端体验\n\n这个目录收纳 Vue 性能优化、交互设计与演示总览。"},
	}
}

func defaultPages() []SeedDoc {
	return []SeedDoc{
		{Key: "go_gc", Title: "Go GC 原理详解", FolderKey: "backend_folder", Tags: []string{"go", "runtime", "gc"}, Markdown: `# Go GC 原理详解

Go 的垃圾回收器是并发标记-清扫（mark-sweep）方案，核心目标是把吞吐、内存占用与暂停时间平衡在一个工程可接受的区间。

在实践中最常用的旋钮是 GOGC。GOGC 提高，通常会减少 GC 触发频率，CPU 压力下降，但峰值内存上升；GOGC 降低则相反。

建议先选一个保守值（如 100），结合 pprof、gctrace 和业务 SLA 逐步迭代。`},
		{Key: "go_context", Title: "Go Context 深入理解", FolderKey: "backend_folder", Tags: []string{"go", "context", "concurrency"}, Markdown: `# Go Context 深入理解

context 包的核心作用是在 goroutine 之间传递取消信号、截止时间和请求范围的值。

context.Background() 返回空的根 context，通常在 main 或测试入口使用。context.WithCancel 派生可取消的子 context，WithTimeout 和 WithDeadline 则增加了时间限制。`},
		{Key: "sqlite_tuning", Title: "SQLite 性能调优", FolderKey: "backend_folder", Tags: []string{"sqlite", "database", "performance"}, Markdown: `# SQLite 性能调优

SQLite 在嵌入式场景中极其强大，但要发挥性能，需要理解它的几个关键配置：WAL 模式、busy_timeout、synchronous 设置和内存映射。`},
		{Key: "rag_basics", Title: "RAG 检索增强生成基础", FolderKey: "ai_folder", Tags: []string{"rag", "ai", "vector"}, Markdown: `# RAG 检索增强生成基础

RAG（Retrieval-Augmented Generation）是将检索与生成结合的架构：先从知识库检索相关文档片段，再将其作为上下文注入 LLM，提高回答的事实性和可控性。`},
		{Key: "embedding", Title: "文本嵌入与向量检索", FolderKey: "ai_folder", Tags: []string{"embedding", "vector", "ai"}, Markdown: `# 文本嵌入与向量检索

文本嵌入将自然语言映射为固定维度的向量，语义相近的文本在向量空间中距离更近。余弦相似度是最常用的距离度量。`},
		{Key: "vue_perf", Title: "Vue 长列表渲染优化", FolderKey: "frontend_folder", Tags: []string{"vue", "performance"}, Markdown: `# Vue 长列表渲染优化

当需要在 Vue 中渲染数千条数据时，虚拟滚动的思想是只渲染可视区域内的 DOM 节点，而不是一次性渲染整个列表。`},
	}
}
