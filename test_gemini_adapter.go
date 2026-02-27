package main

import (
	"context"
	"fmt"
	"log"

	"iron/internal/chat"
	"iron/internal/llm"
	"iron/internal/middleware"
	"github.com/tmc/langchaingo/llms"
)

func main() {
	adapter, err := llm.NewGeminiAdapter("gemini-2.5-flash", "")
	if err != nil { log.Fatal(err) }

	tools := []llms.Tool{
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name: "ls", Description: "list files", Parameters: map[string]any{"type":"object","properties":map[string]any{"path":map[string]any{"type":"string"}}},
			},
		},
	}

	history := []chat.Message{
		{Role: chat.RoleUser, Content: "list the files in the current directory please"},
	}

	params := &middleware.LLMParams{
		Tools: tools,
	}

	content, toolCalls, err := adapter.ReplyStream(context.Background(), history, params, nil)
	if err != nil { log.Fatal(err) }

	fmt.Printf("Content: %s\n", content)
	fmt.Printf("ToolCalls: %+v\n", toolCalls)
}
