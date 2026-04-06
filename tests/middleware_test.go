package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"iron/internal/embedding"
	"iron/internal/nlu"
	"iron/internal/proxy"
	"iron/middlewares/contextcompressor"
	"iron/middlewares/rag"
	"iron/middlewares/semanticcache"
	"iron/middlewares/websearch"
)

// Mock Ollama server for embedding and NLU
func newMockOllamaServer(embedResponse []float32, classifyResponse string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/embeddings") {
			json.NewEncoder(w).Encode(map[string]any{
				"embedding": embedResponse,
			})
		} else {
			// Generate endpoint for classification or compression
			json.NewEncoder(w).Encode(map[string]any{
				"response": classifyResponse,
			})
		}
	}))
}

// Mock ChromaDB server
func newMockChromaServer(queryResult *chromaQueryResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/v1/collections/iron_sem_cache/query" ||
			r.URL.Path == "/api/v1/collections/iron_context/query" {
			json.NewEncoder(w).Encode(queryResult)
		} else {
			json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
}

type chromaQueryResponse struct {
	Texts   []string  `json:"documents"`
	Metrics []float64 `json:"distances"`
}

type testCase struct {
	name          string
	userMessage   string
	middleware    string
	setupMocks    func(t *testing.T) (func(), error)
	verifyFunc    func(t *testing.T, resp *proxy.ChatResponse, req *proxy.ChatRequest)
	skipIfMissing func() bool
}

func TestSemanticCacheMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		userMessage   string
		similarity    float64
		cacheHit      bool
		expectedHit   bool
	}{
		{
			name:        "cache hit with high similarity",
			userMessage: "What is the capital of France?",
			similarity:  0.95,
			cacheHit:    true,
			expectedHit: true,
		},
		{
			name:        "cache miss with low similarity",
			userMessage: "Tell me about cooking pasta",
			similarity:  0.70,
			cacheHit:    false,
			expectedHit: false,
		},
		{
			name:        "no user message returns next",
			userMessage: "",
			similarity:  0.95,
			cacheHit:    false,
			expectedHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var distance float64
			if tt.similarity > 0 {
				distance = 1.0/tt.similarity - 1.0
			}

			cachedResponse := `{"id":"test-id","object":"chat.completion","created":1234567890,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Paris"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`

			chromaServer := newMockChromaServer(&chromaQueryResponse{
				Texts:   []string{cachedResponse},
				Metrics: []float64{distance},
			})
			defer chromaServer.Close()

			ollamaServer := newMockOllamaServer(make([]float32, 768), "simple_query")
			defer ollamaServer.Close()

			embedClient := embedding.NewClient(ollamaServer.URL, "nomic-embed-text")
			mw := semanticcache.New(chromaServer.URL, embedClient, 0.92, 24)

			req := &proxy.ChatRequest{
				Model: "gpt-4",
				Messages: []proxy.ChatMessage{
					{Role: "user", Content: tt.userMessage},
				},
			}

			nextCalled := false
			next := func(r *proxy.ChatRequest) (*proxy.ChatResponse, error) {
				nextCalled = true
				return &proxy.ChatResponse{
					ID:      "fresh-response",
					Model:   "gpt-4",
					Choices: []proxy.Choice{{Message: proxy.ChatMessage{Role: "assistant", Content: "Fresh response"}}},
				}, nil
			}

			resp, err := mw.Handle(req, next)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.userMessage == "" {
				if !nextCalled {
					t.Error("expected next to be called for empty message")
				}
				return
			}

			if tt.expectedHit {
				if nextCalled {
					t.Error("expected cache hit, but next was called")
				}
				if resp.ID != "test-id" {
					t.Errorf("expected cached response id 'test-id', got %s", resp.ID)
				}
			} else {
				if !nextCalled {
					t.Error("expected next to be called on cache miss")
				}
				if resp.ID != "fresh-response" {
					t.Errorf("expected fresh response id 'fresh-response', got %s", resp.ID)
				}
			}
		})
	}
}

func TestRAGMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		userMessage    string
		shouldInjectRAG bool
	}{
		{
			name:           "rag_request intent triggers RAG",
			userMessage:    "What does our documentation say about deployment?",
			shouldInjectRAG: true,
		},
		{
			name:           "previously keyword triggers RAG",
			userMessage:    "What was discussed previously in our meeting notes?",
			shouldInjectRAG: true,
		},
		{
			name:           "from earlier keyword triggers RAG",
			userMessage:    "Summarize from earlier conversations",
			shouldInjectRAG: true,
		},
		{
			name:           "simple query does not trigger RAG",
			userMessage:    "What is the weather today?",
			shouldInjectRAG: false,
		},
		{
			name:           "empty message passes through",
			userMessage:    "",
			shouldInjectRAG: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chromaServer := newMockChromaServer(&chromaQueryResponse{
				Texts:   []string{"Context document about deployment"},
				Metrics: []float64{0.1},
			})
			defer chromaServer.Close()

			ollamaServer := newMockOllamaServer(make([]float32, 768), "rag_request")
			defer ollamaServer.Close()

			embedClient := embedding.NewClient(ollamaServer.URL, "nomic-embed-text")
			nluRouter := nlu.NewRouter(ollamaServer.URL, "llama3.2:1b")

			mw := rag.New(chromaServer.URL, "/tmp", "iron_context", 5, embedClient, nluRouter)

			req := &proxy.ChatRequest{
				Model: "gpt-4",
				Messages: []proxy.ChatMessage{
					{Role: "user", Content: tt.userMessage},
				},
			}

			ragInjected := false
			next := func(r *proxy.ChatRequest) (*proxy.ChatResponse, error) {
				for _, msg := range r.Messages {
					if strings.Contains(msg.Content, "[RAG CONTEXT]") {
						ragInjected = true
					}
				}
				return &proxy.ChatResponse{
					ID:      "rag-response",
					Model:   "gpt-4",
					Choices: []proxy.Choice{{Message: proxy.ChatMessage{Role: "assistant", Content: "RAG response"}}},
				}, nil
			}

			_, err := mw.Handle(req, next)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.shouldInjectRAG && !ragInjected {
				t.Error("expected RAG context to be injected")
			}
			if !tt.shouldInjectRAG && ragInjected {
				t.Error("expected no RAG injection for simple query")
			}
		})
	}
}

func TestWebSearchMiddleware(t *testing.T) {
	tests := []struct {
		name              string
		userMessage       string
		shouldInjectSearch bool
	}{
		{
			name:              "latest keyword triggers web search",
			userMessage:       "What is the latest news on AI?",
			shouldInjectSearch: true,
		},
		{
			name:              "find on the web keyword triggers search",
			userMessage:       "Find on the web about quantum computing",
			shouldInjectSearch: true,
		},
		{
			name:              "search for keyword triggers search",
			userMessage:       "Search for information about climate change",
			shouldInjectSearch: true,
		},
		{
			name:              "simple query does not trigger search",
			userMessage:       "What is 2+2?",
			shouldInjectSearch: false,
		},
		{
			name:              "empty message passes through",
			userMessage:       "",
			shouldInjectSearch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ollamaServer := newMockOllamaServer(make([]float32, 768), "web_search")
			defer ollamaServer.Close()

			nluRouter := nlu.NewRouter(ollamaServer.URL, "llama3.2:1b")

			mw := websearch.New(nluRouter, 5, 10)

			req := &proxy.ChatRequest{
				Model: "gpt-4",
				Messages: []proxy.ChatMessage{
					{Role: "user", Content: tt.userMessage},
				},
			}

			searchInjected := false
			next := func(r *proxy.ChatRequest) (*proxy.ChatResponse, error) {
				for _, msg := range r.Messages {
					if strings.Contains(msg.Content, "[WEB SEARCH RESULTS]") {
						searchInjected = true
					}
				}
				return &proxy.ChatResponse{
					ID:      "search-response",
					Model:   "gpt-4",
					Choices: []proxy.Choice{{Message: proxy.ChatMessage{Role: "assistant", Content: "Search response"}}},
				}, nil
			}

			_, err := mw.Handle(req, next)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.shouldInjectSearch && !searchInjected {
				t.Error("expected web search results to be injected")
			}
			if !tt.shouldInjectSearch && searchInjected {
				t.Error("expected no search injection for simple query")
			}
		})
	}
}

func TestContextCompressorMiddleware(t *testing.T) {
	tests := []struct {
		name             string
		messageCount     int
		avgContentLength int
		threshold        int
		shouldCompress   bool
	}{
		{
			name:             "below threshold does not compress",
			messageCount:     5,
			avgContentLength: 100,
			threshold:        12000,
			shouldCompress:   false,
		},
		{
			name:             "above threshold triggers compression",
			messageCount:     50,
			avgContentLength: 500,
			threshold:        12000,
			shouldCompress:   true,
		},
		{
			name:             "custom threshold respected",
			messageCount:     10,
			avgContentLength: 1000,
			threshold:        1000,
			shouldCompress:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ollamaServer := newMockOllamaServer(make([]float32, 768), "compressed summary")
			defer ollamaServer.Close()

			mw := contextcompressor.New(ollamaServer.URL, "llama3.2:1b", tt.threshold)

			messages := []proxy.ChatMessage{}
			for i := 0; i < tt.messageCount; i++ {
				content := strings.Repeat("x", tt.avgContentLength)
				messages = append(messages, proxy.ChatMessage{
					Role:    "user",
					Content: content,
				})
			}

			req := &proxy.ChatRequest{
				Model:    "gpt-4",
				Messages: messages,
			}

			compressed := false
			next := func(r *proxy.ChatRequest) (*proxy.ChatResponse, error) {
				for _, msg := range r.Messages {
					if strings.Contains(msg.Content, "[COMPRESSED CONTEXT]") {
						compressed = true
					}
				}
				return &proxy.ChatResponse{
					ID:      "compressed-response",
					Model:   "gpt-4",
					Choices: []proxy.Choice{{Message: proxy.ChatMessage{Role: "assistant", Content: "Compressed response"}}},
				}, nil
			}

			_, err := mw.Handle(req, next)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.shouldCompress && !compressed {
				t.Error("expected compression to be applied")
			}
			if !tt.shouldCompress && compressed {
				t.Error("expected no compression below threshold")
			}
		})
	}
}

func TestMiddlewarePipelineOrder(t *testing.T) {
	ollamaServer := newMockOllamaServer(make([]float32, 768), "rag_request")
	defer ollamaServer.Close()

	chromaServer := newMockChromaServer(&chromaQueryResponse{
		Texts:   []string{`{"id":"cached","object":"chat.completion","created":123,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Cached"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`},
		Metrics: []float64{0.05},
	})
	defer chromaServer.Close()

	embedClient := embedding.NewClient(ollamaServer.URL, "nomic-embed-text")
	nluRouter := nlu.NewRouter(ollamaServer.URL, "llama3.2:1b")

	semanticCacheMw := semanticcache.New(chromaServer.URL, embedClient, 0.92, 24)
	ragMw := rag.New(chromaServer.URL, "/tmp", "iron_context", 5, embedClient, nluRouter)
	webSearchMw := websearch.New(nluRouter, 5, 10)
	contextCompMw := contextcompressor.New(ollamaServer.URL, "llama3.2:1b", 12000)

	pipeline := proxy.NewPipeline(
		semanticCacheMw,
		ragMw,
		webSearchMw,
		contextCompMw,
	)

	executionOrder := []string{}
	makeRecorder := func(name string) func(*proxy.ChatRequest) (*proxy.ChatResponse, error) {
		return func(r *proxy.ChatRequest) (*proxy.ChatResponse, error) {
			executionOrder = append(executionOrder, name)
			return &proxy.ChatResponse{
				ID:      "final",
				Model:   "gpt-4",
				Choices: []proxy.Choice{{Message: proxy.ChatMessage{Role: "assistant", Content: "Final"}}},
			}, nil
		}
	}

	req := &proxy.ChatRequest{
		Model: "gpt-4",
		Messages: []proxy.ChatMessage{
			{Role: "user", Content: "What does our docs say about deployment?"},
		},
	}

	final := makeRecorder("final")
	_, err := pipeline.Execute(req, final)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOrder := []string{"semanticcache", "rag", "websearch", "contextcompressor", "final"}

	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("expected %d middleware executions, got %d", len(expectedOrder), len(executionOrder))
		t.Errorf("execution order: %v", executionOrder)
	}

	for i, expected := range expectedOrder {
		if i < len(executionOrder) && executionOrder[i] != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
}
