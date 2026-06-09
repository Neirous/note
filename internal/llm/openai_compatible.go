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

type OpenAICompatibleClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	embedModel string
	chatModel  string
}

func NewOpenAICompatibleClient(httpClient *http.Client, baseURL, apiKey, embedModel, chatModel string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     strings.TrimSpace(apiKey),
		embedModel: embedModel,
		chatModel:  chatModel,
	}
}

func (c *OpenAICompatibleClient) Embed(ctx context.Context, text string) ([]float64, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is empty")
	}

	type reqBody struct {
		Model          string `json:"model"`
		Input          string `json:"input"`
		EncodingFormat string `json:"encoding_format,omitempty"`
	}
	type respBody struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	body, _ := json.Marshal(reqBody{
		Model:          c.embedModel,
		Input:          text,
		EncodingFormat: "float",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.withAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embeddings: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings status %d: %s", resp.StatusCode, string(raw))
	}

	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, fmt.Errorf("embeddings error: %s", out.Error.Message)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embeddings returned empty vector")
	}
	return out.Data[0].Embedding, nil
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is empty")
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Temperature float64   `json:"temperature,omitempty"`
		Stream      bool      `json:"stream"`
	}
	type respBody struct {
		Choices []struct {
			Message message `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	body, _ := json.Marshal(reqBody{
		Model: c.chatModel,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		Stream:      false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.withAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call chat completions: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat completions status %d: %s", resp.StatusCode, string(raw))
	}

	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("chat completions error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("chat completions returned empty choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (c *OpenAICompatibleClient) withAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
}
