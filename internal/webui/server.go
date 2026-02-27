package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"iron/internal/chat"
	"iron/internal/gateway"
	"iron/internal/onboarding"
)

//go:embed static
var staticFiles embed.FS

// Server represents the Web UI backend server
type Server struct {
	gw      *gateway.Gateway
	service *chat.Service
	mu      sync.Mutex
	port    int
	cleanup func()
}

// NewServer creates a new Web UI server instance
func NewServer(gw *gateway.Gateway, port int) *Server {
	if port == 0 {
		port = 8080
	}
	return &Server{
		gw:   gw,
		port: port,
	}
}

// Start initializes the service and starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Initialize the chat service for the Web UI
	// Using a no-op stream callback so it doesn't spam stdout
	streamCb := func(msg string) {}

	service, _, _, _, cleanup, err := s.gw.InitService(ctx, chat.WithStreamCallback(streamCb))
	if err != nil {
		return fmt.Errorf("failed to init service for webui: %w", err)
	}
	s.service = service
	s.cleanup = cleanup
	defer s.cleanup()

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/status", s.handleStatus)

	// Serve static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to load static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		log.Println("[WebUI] Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("[WebUI] 🌐 Starting Web UI on http://localhost:%d", s.port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webui server error: %w", err)
	}

	return nil
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
	Error string `json:"error,omitempty"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure thread-safe access to the single chat session for now
	s.mu.Lock()
	defer s.mu.Unlock()

	turnCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	reply, err := s.service.Send(turnCtx, req.Message)

	resp := ChatResponse{Reply: reply}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	configPath := s.gw.ConfigPath
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".iron", "config.json")
	}

	if r.Method == http.MethodGet {
		cfg, err := onboarding.LoadFromFile(configPath)
		if err != nil {
			// Return an empty config if it doesn't exist yet
			cfg = &onboarding.Config{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == http.MethodPost {
		var cfg onboarding.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			http.Error(w, "Failed to marshal config", http.StatusInternalServerError)
			return
		}

		os.MkdirAll(filepath.Dir(configPath), 0755)
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		// Reload gateway config in-memory
		s.gw.LoadConfig()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"status": "online",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
