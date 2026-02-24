package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iron/internal/chat"
	"iron/internal/llm"
	"iron/internal/middleware"
	_ "iron/middlewares/autoload"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	ctx := context.Background()

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
		fmt.Fprintf(os.Stderr, "failed to initialize ollama client: %v\n", err)
		os.Exit(1)
	}

	// Middleware debug log (JSONL), always on by default.
	logPath := filepath.Join("bin", "middleware.debug.jsonl")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to open middleware log file (%s): %v\n", logPath, err)
	}
	var mwLog io.Writer = logFile

	chain := middleware.NewChainFromRegistry(mwLog)

	var service *chat.Service
	if chain != nil {
		service = chat.NewService(adapter, chat.WithMiddlewareChain(chain))
	} else {
		service = chat.NewService(adapter)
	}

	fmt.Println("IRon chat (LangChain Go + Ollama)")
	fmt.Printf("model=%s (set IRON_MODEL), ollama_url=%s\n", model, valueOrDefault(baseURL, "http://localhost:11434"))
	fmt.Println("Type /exit to quit, /clear to reset context.")
	fmt.Printf("Middleware debug log: %s\n", logPath)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch input {
		case "/exit", "exit", "quit":
			return
		case "/clear":
			service.Clear()
			fmt.Println("context cleared")
			continue
		}

		turnCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		assistant, err := service.Send(turnCtx, input)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		fmt.Println(assistant)
	}
}

func valueOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
