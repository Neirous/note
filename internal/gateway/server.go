package gateway

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"

	"note/internal/api"
	"note/internal/llm"
	"note/internal/rag"
	"note/internal/store"
)

var (
	GitBranch = "unknown"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

type modelClient interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	Generate(ctx context.Context, prompt string) (string, error)
}

func RunServer(addr, dsn, provider string) {
	log.Infof("note server [%s] %s built at %s", GitBranch, GitCommit, BuildDate)

	if addr == "" {
		addr = getenv("APP_ADDR", ":8080")
	}
	if dsn == "" {
		dsn = getenv("APP_DSN", "file:notes.db?_pragma=busy_timeout(5000)")
	}
	if provider == "" {
		provider = strings.ToLower(getenv("LLM_PROVIDER", "dashscope"))
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := store.New(db)
	if err := s.InitSchema(context.Background()); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	provider, mc := buildModelClient(provider, httpClient)
	if provider == "dashscope" && os.Getenv("OPENAI_API_KEY") == "" {
		log.Warn("OPENAI_API_KEY is empty, RAG will fail until it is configured")
	}

	log.Infof("llm provider: %s", provider)
	ragService := rag.NewService(s, mc, mc, rag.Config{
		MaxChunkChars: 800,
		TopK:          5,
	})

	handler := api.NewServer(s, ragService)

	server := &http.Server{
		Addr:    addr,
		Handler: handler.Routes(),
	}

	go func() {
		log.Infof("server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Infof("received signal %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("shutdown error: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func buildModelClient(provider string, httpClient *http.Client) (string, modelClient) {
	switch provider {
	case "dashscope":
		baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
		apiKey := os.Getenv("OPENAI_API_KEY")
		embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
		chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")
		return "dashscope", llm.NewOpenAICompatibleClient(httpClient, baseURL, apiKey, embedModel, chatModel)
	case "ollama":
		baseURL := getenv("OLLAMA_BASE_URL", "http://localhost:11434")
		embedModel := getenv("OLLAMA_EMBED_MODEL", "nomic-embed-text")
		genModel := getenv("OLLAMA_GEN_MODEL", "qwen2.5:7b")
		return "ollama", llm.NewOllamaClient(httpClient, baseURL, embedModel, genModel)
	default:
		log.Warnf("unknown LLM_PROVIDER=%q, fallback to dashscope", provider)
		baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
		apiKey := os.Getenv("OPENAI_API_KEY")
		embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
		chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")
		return "dashscope", llm.NewOpenAICompatibleClient(httpClient, baseURL, apiKey, embedModel, chatModel)
	}
}
