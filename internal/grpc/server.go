package grpcserver

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"note/internal/llm"
	"note/internal/rag"
	"note/internal/store"

	notepb "note/api/proto/note/v1"
	intelligencepb "note/api/proto/intelligence/v1"
	knowledgepb "note/api/proto/knowledge/v1"
	workspacepb "note/api/proto/workspace/v1"
	ragpb "note/api/proto/rag/v1"
)

func Run(addr, dsn, provider string) {
	log.Infof("gRPC server starting on %s", addr)

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

	st := store.New(db)
	if err := st.InitSchema(context.Background()); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	provider, mc := buildModelClient(provider, httpClient)
	if provider == "dashscope" && os.Getenv("OPENAI_API_KEY") == "" {
		log.Warn("OPENAI_API_KEY is empty, intelligence/RAG services will fail")
	}

	log.Infof("llm provider: %s", provider)
	ragService := rag.NewService(st, mc, mc, rag.Config{MaxChunkChars: 800, TopK: 5})

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	srv := grpc.NewServer()

	notepb.RegisterNoteServiceServer(srv, NewNoteServer(st, ragService))
	knowledgepb.RegisterKnowledgeServiceServer(srv, NewKnowledgeServer(st))
	ragpb.RegisterRAGServiceServer(srv, NewRAGServer(ragService))
	intelligencepb.RegisterIntelligenceServiceServer(srv, NewIntelligenceServer(st, ragService))
	workspacepb.RegisterWorkspaceServiceServer(srv, NewWorkspaceServer(st, ragService))

	reflection.Register(srv)

	log.Infof("gRPC listening on %s", addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type modelClient interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	Generate(ctx context.Context, prompt string) (string, error)
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
