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
	"strings"
	"sync"
	"time"

	"iron/internal/chat"
	"iron/internal/gateway"
	"iron/internal/middleware"
	"iron/internal/onboarding"
)

//go:embed static
var staticFiles embed.FS

// Session holds the state for a single Web UI chat session
type Session struct {
	service *chat.Service
	cleanup func()
	eventCh chan string
}

// Server represents the Web UI backend server
type Server struct {
	gw       *gateway.Gateway
	sessions map[string]*Session
	mu       sync.Mutex
	port     int
}

// NewServer creates a new Web UI server instance
func NewServer(gw *gateway.Gateway, port int) *Server {
	if port == 0 {
		port = 8080
	}
	return &Server{
		gw:       gw,
		port:     port,
		sessions: make(map[string]*Session),
	}
}

func (s *Server) getSession(ctx context.Context, id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}

	eventCh := make(chan string, 200)

	streamCb := func(msg string) {
		select {
		case eventCh <- msg:
		default:
		}
	}

	statusCb := func(msg string) {
		select {
		case eventCh <- "[STATUS] " + msg:
		default:
		}
	}

	service, _, _, _, cleanup, err := s.gw.InitService(ctx, chat.WithStreamCallback(streamCb), chat.WithStatusCallback(statusCb))
	if err != nil {
		return nil, err
	}

	sess := &Session{
		service: service,
		cleanup: cleanup,
		eventCh: eventCh,
	}
	s.sessions[id] = sess
	return sess, nil
}

// Start initializes the service and starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/plugins", s.handlePlugins)

	// Serve static files (handling SPA routing)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to load static files: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Check if the file exists in the embed FS
		f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// If the file doesn't exist (e.g. /dashboard or nested route), serve index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

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

		s.mu.Lock()
		for _, sess := range s.sessions {
			sess.cleanup()
		}
		s.mu.Unlock()
	}()

	log.Printf("[WebUI] 🌐 Starting Web UI on http://localhost:%d", s.port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webui server error: %w", err)
	}

	return nil
}

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
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

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	sess, err := s.getSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	turnCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	reply, err := sess.service.Send(turnCtx, req.Message)

	resp := ChatResponse{Reply: reply}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = "default"
	}

	sess, err := s.getSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sess.eventCh:
			payload, _ := json.Marshal(map[string]string{"text": msg})
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
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

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	registered := middleware.Registered()
	plugins := make([]map[string]interface{}, 0, len(registered))

	for _, mw := range registered {
		plugins = append(plugins, map[string]interface{}{
			"id":       mw.ID(),
			"priority": mw.Priority(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"plugins": plugins,
	})
}
