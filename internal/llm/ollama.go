package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OllamaClient struct {
	httpClient *http.Client
	baseURL    string
	embedModel string
	genModel   string
}

func NewOllamaClient(httpClient *http.Client, baseURL, embedModel, genModel string) *OllamaClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &OllamaClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		embedModel: embedModel,
		genModel:   genModel,
	}
}

func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float64, error) {
	type reqBody struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}
	type respBody struct {
		Embeddings [][]float64 `json:"embeddings"`
		Embedding  []float64   `json:"embedding"` // backward compatibility
		Error      string      `json:"error"`
	}

	body, _ := json.Marshal(reqBody{Model: c.embedModel, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama embed: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return c.embedLegacy(ctx, text)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed status %d: %s", resp.StatusCode, string(raw))
	}

	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama embed: %s", out.Error)
	}
	if len(out.Embeddings) > 0 && len(out.Embeddings[0]) > 0 {
		return out.Embeddings[0], nil
	}
	if len(out.Embedding) > 0 {
		return out.Embedding, nil
	}
	return nil, fmt.Errorf("ollama embed returned empty vector")
}

func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	type reqBody struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}
	type respBody struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}

	body, _ := json.Marshal(reqBody{Model: c.genModel, Prompt: prompt, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ollama generate: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama generate status %d: %s", resp.StatusCode, string(raw))
	}

	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama generate: %s", out.Error)
	}
	return strings.TrimSpace(out.Response), nil
}

func (c *OllamaClient) embedLegacy(ctx context.Context, text string) ([]float64, error) {
	type reqBody struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	type respBody struct {
		Embedding []float64 `json:"embedding"`
		Error     string    `json:"error"`
	}

	body, _ := json.Marshal(reqBody{Model: c.embedModel, Prompt: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama embeddings: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embeddings status %d: %s", resp.StatusCode, string(raw))
	}
	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama embeddings: %s", out.Error)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embeddings returned empty vector")
	}
	return out.Embedding, nil
}
