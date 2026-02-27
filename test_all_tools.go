package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"iron/internal/middleware"
	_ "iron/middlewares/autoload" // load all middlewares
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func main() {
	chain := middleware.NewChainFromRegistry(nil)
	if chain == nil { log.Fatal("no chain") }
	e := &middleware.Event{
		Name: middleware.EventBeforeLLMRequest,
		Params: &middleware.LLMParams{},
		Context: map[string]any{},
	}
	chain.Dispatch(context.Background(), e)

	fmt.Printf("Number of tools from middlewares: %d\n", len(e.Params.Tools))

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" { key = os.Getenv("IRON_GEMINI_API_KEY") }

	client, err := openai.New(
		openai.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
		openai.WithToken(key),
		openai.WithModel("gemini-2.5-flash"),
	)
	if err != nil { log.Fatal(err) }

	resp, err := client.GenerateContent(context.Background(), []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "list the files in the current directory please"),
	}, llms.WithTools(e.Params.Tools), llms.WithToolChoice("required"))
	
	if err != nil {
		log.Fatalf("GenerateContent Error: %v", err)
	}

	fmt.Printf("Success! Choice: %+v\n", resp.Choices[0])
	for _, tc := range resp.Choices[0].ToolCalls {
		fmt.Printf("ToolCall: %s %v\n", tc.FunctionCall.Name, tc.FunctionCall.Arguments)
	}
}
