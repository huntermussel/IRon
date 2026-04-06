package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewClient(baseURL, model string) *Client {
	return &Client{baseURL: baseURL, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

type EmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

func (c *Client) Embed(ctx context.Context, text string) ([]float64, error) {
	body, _ := json.Marshal(map[string]any{"model": c.model, "prompt": text})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i, text := range texts {
		emb, err := c.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("batch embed %d: %w", i, err)
		}
		results[i] = emb
	}
	return results, nil
}
