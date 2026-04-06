package nlu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type QueryIntent string

const (
	QueryIntentSimpleQuery QueryIntent = "simple_query"
	QueryIntentComplexTask QueryIntent = "complex_task"
	QueryIntentRAGRequest  QueryIntent = "rag_request"
	QueryIntentWebSearch   QueryIntent = "web_search"
	QueryIntentCodeGen     QueryIntent = "code_generation"
)

type ClassifyResult struct {
	Intent    QueryIntent      `json:"intent"`
	Confidence float64         `json:"confidence"`
	Reasoning string           `json:"reasoning"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

type Router struct {
	ollamaURL string
	fastModel string
}

func NewRouter(ollamaURL, fastModel string) *Router {
	return &Router{ollamaURL: ollamaURL, fastModel: fastModel}
}

var intentKeywords = map[QueryIntent][]string{
	QueryIntentRAGRequest:  {"context", "knowledge base", "documentation", "previously", "remember", "from earlier", "stored"},
	QueryIntentWebSearch:   {"current", "latest", "today", "news", "weather", "search for", "find on the web"},
	QueryIntentCodeGen:     {"write code", "generate", "implement", "create function", "write a", "build a"},
	QueryIntentComplexTask: {"plan", "analyze", "compare", "design", "strategy", "research", "evaluate"},
}

var codePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(python|javascript|go|rust|typescript|java)\b`),
	regexp.MustCompile(`(?i)\b(write|generate|create|implement|code)\b`),
}

func (r *Router) Classify(ctx context.Context, text string) (*ClassifyResult, error) {
	lower := strings.ToLower(text)
	for intent, keywords := range intentKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return &ClassifyResult{Intent: intent, Confidence: 0.7, Reasoning: fmt.Sprintf("keyword match: %q", kw)}, nil
			}
		}
	}
	for _, pat := range codePatterns {
		if pat.MatchString(text) {
			return &ClassifyResult{Intent: QueryIntentCodeGen, Confidence: 0.75, Reasoning: "code pattern detected"}, nil
		}
	}

	prompt := fmt.Sprintf(`Classify into one of: simple_query, complex_task, rag_request, web_search, code_generation.\nQuery: %s\nRespond with only the intent label.`, text)
	body, _ := json.Marshal(map[string]any{"model": r.fastModel, "prompt": prompt, "stream": false})
	req, _ := http.NewRequestWithContext(ctx, "POST", r.ollamaURL+"/api/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ClassifyResult{Intent: QueryIntentSimpleQuery, Confidence: 0.5, Reasoning: "Ollama unavailable"}, nil
	}
	defer resp.Body.Close()
	var result struct{ Response string }
	json.NewDecoder(resp.Body).Decode(&result)
	return &ClassifyResult{Intent: QueryIntent(strings.TrimSpace(strings.ToLower(result.Response))), Confidence: 0.65, Reasoning: "Ollama classification"}, nil
}
