package rag

import (
	"context"
	"fmt"
	"log"

	"iron/internal/chroma"
	"iron/internal/embedding"
	"iron/internal/nlu"
	"iron/internal/proxy"
)

type RAGMiddleware struct {
	chroma  *chroma.Client
	embed   *embedding.Client
	cfg     proxy.RAGConfig
	nlu     *nlu.Router
}

func New(chromaURL, chromaPath, collection string, topK int, embed *embedding.Client, nlu *nlu.Router) *RAGMiddleware {
	return &RAGMiddleware{
		chroma: chroma.NewClient(chromaURL),
		embed:  embed,
		cfg:    proxy.RAGConfig{ChromaPath: chromaPath, DefaultCollection: collection, TopK: topK},
		nlu:    nlu,
	}
}

func (m *RAGMiddleware) Name() string { return "rag" }

func (m *RAGMiddleware) Handle(req *proxy.ChatRequest, next func(*proxy.ChatRequest) (*proxy.ChatResponse, error)) (*proxy.ChatResponse, error) {
	ctx := context.Background()
	query := lastUserMessage(req)
	if query == "" {
		return next(req)
	}
	intent, _ := m.nlu.Classify(ctx, query)
	if intent == nil || intent.Intent != nlu.QueryIntentRAGRequest {
		return next(req)
	}
	emb, err := m.embed.Embed(ctx, query)
	if err != nil {
		log.Printf("[rag] embedding failed: %v", err)
		return next(req)
	}
	result, err := m.chroma.Query(ctx, m.cfg.DefaultCollection, emb, m.cfg.TopK)
	if err != nil {
		log.Printf("[rag] query failed: %v", err)
		return next(req)
	}
	if len(result.Texts) == 0 {
		return next(req)
	}
	contextStr := formatContext(result)
	req.Messages = append(req.Messages, proxy.ChatMessage{Role: "system", Content: contextStr})
	return next(req)
}

func formatContext(r *chroma.QueryResult) string {
	b := "[RAG CONTEXT]\n"
	for i, text := range r.Texts {
		b += fmt.Sprintf("Source %d: %s\n", i+1, text)
	}
	b += "[/RAG CONTEXT]\n"
	return b
}

func lastUserMessage(req *proxy.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}
