package chroma

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, client: &http.Client{Timeout: 30 * time.Second}}
}

type QueryResult struct {
	IDs     []string  `json:"ids"`
	Texts   []string  `json:"texts"`
	Metrics []float64 `json:"distances,omitempty"`
}

func (c *Client) CreateCollection(ctx context.Context, name string) error {
	body, _ := json.Marshal(map[string]any{"name": name, "get_or_create": true})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v2/collections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/v2/collections/"+name, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) AddDocument(ctx context.Context, collectionName, id, text string, embedding []float64) error {
	payload := map[string]any{
		"ids":        []string{id},
		"documents":  []string{text},
		"embeddings": [][]float64{embedding},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v2/collections/"+collectionName+"/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Query(ctx context.Context, collectionName string, queryEmbedding []float64, n int) (*QueryResult, error) {
	payload := map[string]any{
		"query_embeddings": [][]float64{queryEmbedding},
		"n_results":        n,
		"include":          []string{"documents", "metadatas"},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v2/collections/"+collectionName+"/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		IDs       [][]string  `json:"ids"`
		Documents [][]string  `json:"documents"`
		Distances [][]float64 `json:"distances,omitempty"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.IDs) == 0 || len(result.IDs[0]) == 0 {
		return &QueryResult{}, nil
	}
	return &QueryResult{IDs: result.IDs[0], Texts: result.Documents[0], Metrics: result.Distances[0]}, nil
}
