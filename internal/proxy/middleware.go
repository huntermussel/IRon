package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Middleware interface {
	Name() string
	Handle(req *ChatRequest, next func(*ChatRequest) (*ChatResponse, error)) (*ChatResponse, error)
}

type Pipeline struct {
	middlewares []Middleware
}

func NewPipeline(m ...Middleware) *Pipeline {
	return &Pipeline{middlewares: m}
}

func (p *Pipeline) Execute(req *ChatRequest, final func(*ChatRequest) (*ChatResponse, error)) (*ChatResponse, error) {
	handler := final
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		mw := p.middlewares[i]
		next := handler
		handler = func(r *ChatRequest) (*ChatResponse, error) {
			return mw.Handle(r, next)
		}
	}
	return handler(req)
}

type ProxyServer struct {
	config *Config
	pipeline *Pipeline
	client  *http.Client
}

func NewServer(cfg *Config) *ProxyServer {
	return &ProxyServer{
		config: cfg,
		client: &http.Client{},
	}
}

func (s *ProxyServer) SetupPipelineWith(mws ...Middleware) {
	s.pipeline = NewPipeline(mws...)
	log.Printf("[proxy] pipeline initialized with %d middlewares", len(mws))
}

func (s *ProxyServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/health", s.handleHealth)
}

func (s *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := s.pipeline.Execute(&req, func(cr *ChatRequest) (*ChatResponse, error) {
		return s.forwardToUpstream(cr, r)
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *ProxyServer) forwardToUpstream(req *ChatRequest, r *http.Request) (*ChatResponse, error) {
	bodyBytes, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", s.config.Proxy.Upstream+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.config.Proxy.APIKeyHeader != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.Header.Get(s.config.Proxy.APIKeyHeader))
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}
	return &chatResp, nil
}

func (s *ProxyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"status":"ok"}`)
}
