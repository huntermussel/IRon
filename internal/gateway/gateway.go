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
	"iron/internal/onboarding"
	"iron/internal/skills"
	_ "iron/middlewares/autoload" // Auto-load all middlewares

	"github.com/joho/godotenv"
)

type Gateway struct {
	ConfigPath string
}

func New(configPath string) *Gateway {
	return &Gateway{ConfigPath: configPath}
}

func (g *Gateway) initService(ctx context.Context) (*chat.Service, string, llm.Provider, string, func(), error) {
	// Load environment variables from .env if present
	_ = godotenv.Load()

	// Default values
	model := "llama3.2"
	provider := llm.ProviderOllama
	baseURL := ""
	apiKey := ""

	// Load from config file if available
	if g.ConfigPath != "" {
		if cfg, err := onboarding.LoadFromFile(g.ConfigPath); err == nil {
			model = cfg.Model
			provider = llm.Provider(cfg.Provider)
			baseURL = cfg.BaseURL
			apiKey = cfg.APIKey

			// Apply middleware settings
			var disabled []string
			for _, m := range cfg.Middlewares {
				if !m.Enabled {
					disabled = append(disabled, m.ID)
				}
				for k, v := range m.EnvVars {
					if v != "" {
						os.Setenv(k, v)
					}
				}
			}
			if len(disabled) > 0 {
				os.Setenv("IRON_DISABLED_MIDDLEWARES", strings.Join(disabled, ","))
			}
		}
	}

	// Environment variables override config file
	if m := os.Getenv("IRON_MODEL"); m != "" {
		model = m
	}
	if p := os.Getenv("IRON_PROVIDER"); p != "" {
		provider = llm.Provider(p)
	}
	if u := os.Getenv("IRON_OLLAMA_URL"); u != "" {
		baseURL = u
	}
	if apiKey != "" {
		keyVar := "IRON_" + strings.ToUpper(string(provider)) + "_API_KEY"
		if os.Getenv(keyVar) == "" {
			os.Setenv(keyVar, apiKey)
		}
	}

	// 1. Initialize Components
	adapter, err := llm.NewAdapter(provider, model, baseURL)
	if err != nil {
		return nil, "", "", "", nil, fmt.Errorf("failed to initialize adapter: %w", err)
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

	cleanup := func() {
		browserCtrl.Stop()
	}

	return service, model, provider, baseURL, cleanup, nil
}

func (g *Gateway) Execute(ctx context.Context, input string) error {
	service, _, _, _, cleanup, err := g.initService(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	turnCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	_, err = service.Send(turnCtx, input)
	if err != nil {
		return err
	}
	fmt.Println()
	return nil
}

func (g *Gateway) Run(ctx context.Context) error {
	service, model, provider, baseURL, cleanup, err := g.initService(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

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
