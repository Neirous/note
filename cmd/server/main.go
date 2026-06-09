package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"note/internal/api"
	"note/internal/llm"
	"note/internal/rag"
	"note/internal/store"
)

type modelClient interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	Generate(ctx context.Context, prompt string) (string, error)
}

func main() {
	addr := getenv("APP_ADDR", ":8080")
	dsn := getenv("APP_DSN", "file:notes.db?_pragma=busy_timeout(5000)")
	provider := strings.ToLower(getenv("LLM_PROVIDER", "dashscope"))

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
	provider, modelClient := buildModelClient(provider, httpClient)
	if provider == "dashscope" && os.Getenv("OPENAI_API_KEY") == "" {
		log.Printf("warning: OPENAI_API_KEY is empty, RAG will fail until it is configured")
	}

	log.Printf("llm provider: %s", provider)
	ragService := rag.NewService(s, modelClient, modelClient, rag.Config{
		MaxChunkChars: 800,
		TopK:          5,
	})

	handler := api.NewServer(s, ragService)

	server := &http.Server{
		Addr:    addr,
		Handler: handler.Routes(),
	}

	go func() {
		log.Printf("server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
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
		log.Printf("warning: unknown LLM_PROVIDER=%q, fallback to dashscope", provider)
		baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
		apiKey := os.Getenv("OPENAI_API_KEY")
		embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
		chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")
		return "dashscope", llm.NewOpenAICompatibleClient(httpClient, baseURL, apiKey, embedModel, chatModel)
	}
}
