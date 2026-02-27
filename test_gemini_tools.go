package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func funcTool(name, desc string, params any) llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	}
}

func main() {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" { key = os.Getenv("IRON_GEMINI_API_KEY") }

	client, err := openai.New(
		openai.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
		openai.WithToken(key),
		openai.WithModel("gemini-2.5-flash"),
	)
	if err != nil { log.Fatal(err) }

	tools := []llms.Tool{
		funcTool("ls", "List files", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
			"required": []string{"path"},
		}),
		funcTool("pwd", "Show cwd", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}),
	}

	resp, err := client.GenerateContent(context.Background(), []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "list the files in the current directory please"),
	}, llms.WithTools(tools), llms.WithToolChoice("auto"))
	if err != nil { log.Fatal(err) }

	b, _ := json.MarshalIndent(resp.Choices[0], "", "  ")
	fmt.Println(string(b))
}
