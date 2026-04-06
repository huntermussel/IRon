package semanticcache

import (
	"context"
	"encoding/json"
	"log"

	"iron/internal/chroma"
	"iron/internal/embedding"
	"iron/internal/proxy"
)

type SemanticCache struct {
	chroma    *chroma.Client
	embed     *embedding.Client
	threshold float64
	ttlHours  int
}

func New(chromaURL string, embed *embedding.Client, threshold float64, ttlHours int) *SemanticCache {
	return &SemanticCache{chroma: chroma.NewClient(chromaURL), embed: embed, threshold: threshold, ttlHours: ttlHours}
}

func (m *SemanticCache) Name() string { return "semanticcache" }

func (m *SemanticCache) Handle(req *proxy.ChatRequest, next func(*proxy.ChatRequest) (*proxy.ChatResponse, error)) (*proxy.ChatResponse, error) {
	ctx := context.Background()
	query := lastUserMessage(req)
	if query == "" {
		return next(req)
	}
	emb, err := m.embed.Embed(ctx, query)
	if err != nil {
		log.Printf("[semanticcache] embed failed: %v", err)
		return next(req)
	}
	result, err := m.chroma.Query(ctx, "iron_sem_cache", emb, 1)
	if err != nil || len(result.Texts) == 0 {
		return next(req)
	}
	similarity := 1.0
	if len(result.Metrics) > 0 && result.Metrics[0] > 0 {
		similarity = 1.0 / (1.0 + result.Metrics[0])
	}
	if similarity < m.threshold {
		return next(req)
	}
	log.Printf("[semanticcache] cache hit, similarity=%.3f", similarity)
	var cached proxy.ChatResponse
	if err := json.Unmarshal([]byte(result.Texts[0]), &cached); err != nil {
		log.Printf("[semanticcache] parse failed: %v", err)
		return next(req)
	}
	return &cached, nil
}

func lastUserMessage(req *proxy.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}
