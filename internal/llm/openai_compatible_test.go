package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleClientEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("missing auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer srv.Close()

	c := NewOpenAICompatibleClient(srv.Client(), srv.URL, "test-key", "text-embedding-v3", "qwen-plus")
	v, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("embed error: %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("expected embedding size 3, got %d", len(v))
	}
}

func TestOpenAICompatibleClientGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("missing auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  hi  "}}]}`))
	}))
	defer srv.Close()

	c := NewOpenAICompatibleClient(srv.Client(), srv.URL, "test-key", "text-embedding-v3", "qwen-plus")
	resp, err := c.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}
	if resp != "hi" {
		t.Fatalf("expected trimmed content, got %q", resp)
	}
}

func TestOpenAICompatibleClientEmptyAPIKey(t *testing.T) {
	c := NewOpenAICompatibleClient(http.DefaultClient, "http://x", "", "text-embedding-v3", "qwen-plus")
	if _, err := c.Embed(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected api key error, got %v", err)
	}
}
