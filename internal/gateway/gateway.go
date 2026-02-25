package gateway

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"iron/internal/browser"
	"iron/internal/chat"
	"iron/internal/llm"
	"iron/internal/memory"
	"iron/internal/middleware"
	"iron/internal/skills"
	_ "iron/middlewares/autoload" // Auto-load all middlewares
)

type Gateway struct {
	// Config if we had a struct, for now using env/defaults
}

func New() *Gateway {
	return &Gateway{}
}

func (g *Gateway) Run(ctx context.Context) error {
	// 1. Initialize Components
	model := os.Getenv("IRON_MODEL")
	if model == "" {
		model = "llama3.2"
	}

	provider := llm.Provider(os.Getenv("IRON_PROVIDER"))
	if provider == "" {
		provider = llm.ProviderOllama
	}

	baseURL := os.Getenv("IRON_OLLAMA_URL")
	adapter, err := llm.NewAdapter(provider, model, baseURL)
	if err != nil {
		return fmt.Errorf("failed to initialize adapter: %w", err)
	}

	// Middleware logging
	logPath := filepath.Join("bin", "middleware.debug.jsonl")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to open middleware log file (%s): %v\n", logPath, err)
	}
	var mwLog io.Writer = logFile

	chain := middleware.NewChainFromRegistry(mwLog)
	memStore := memory.NewStore()

	// Browser
	browserCfg := browser.Config{
		Enabled:  true,
		Headless: true,
	}
	browserCtrl := browser.New(browserCfg)
	if err := browserCtrl.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to start browser: %v\n", err)
	}
	defer browserCtrl.Stop()

	// Skills
	skillMgr := skills.NewManager()
	skillMgr.Register(&skills.ShellSkill{})
	skillMgr.Register(&skills.FileSkill{})
	skillMgr.Register(&skills.FetchSkill{})
	skillMgr.Register(&skills.MemorySkill{Store: memStore})
	skillMgr.Register(&skills.BrowserSkill{Controller: browserCtrl})

	opts := []chat.ServiceOption{
		chat.WithMiddlewareChain(chain),
		chat.WithMemoryStore(memStore),
		chat.WithSkills(skillMgr),
	}

	service := chat.NewService(adapter, opts...)

	// 2. Interactive Loop
	fmt.Println("IRon chat")
	fmt.Printf("model=%s, provider=%s, url=%s\n", model, provider, valueOrDefault(baseURL, "default"))
	fmt.Println("Type /exit to quit, /clear to reset context.")
	fmt.Println("Skills loaded: Shell, File, Fetch, Memory, Browser")

	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		<-ctx.Done()
		os.Stdin.Close() // Force read error to break loop
	}()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			return nil
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch input {
		case "/exit", "exit", "quit":
			return nil
		case "/clear":
			service.Clear()
			fmt.Println("context cleared")
			continue
		}

		turnCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		assistant, err := service.Send(turnCtx, input)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		// Output is streamed, so we don't print result again unless needed
		_ = assistant
	}
}

func valueOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
