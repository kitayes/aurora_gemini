package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	EmbeddingModel = "text-embedding-004"
	EmbeddingDims  = 768
	MaxBatchSize   = 100
)

type Service interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type GeminiEmbedder struct {
	apiKey string
	client *http.Client
}

func NewGeminiEmbedder(apiKey string) *GeminiEmbedder {
	return &GeminiEmbedder{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model   string `json:"model"`
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

type embeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type batchEmbeddingRequest struct {
	Requests []embeddingRequest `json:"requests"`
}

type batchEmbeddingResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text provided")
	}

	req := embeddingRequest{
		Model: EmbeddingModel,
	}
	req.Content.Parts = []struct {
		Text string `json:"text"`
	}{{Text: text}}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s",
		EmbeddingModel, g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (code %d)", embResp.Error.Message, embResp.Error.Code)
	}

	if len(embResp.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return embResp.Embedding.Values, nil
}

func (g *GeminiEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts slice")
	}

	if len(texts) > MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(texts), MaxBatchSize)
	}

	results := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := g.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		results[i] = embedding
	}

	return results, nil
}
