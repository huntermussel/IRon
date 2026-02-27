package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func main() {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" { key = os.Getenv("GOOGLE_API_KEY") }
	if key == "" { key = os.Getenv("IRON_GEMINI_API_KEY") }

	client, err := openai.New(
		openai.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
		openai.WithToken(key),
		openai.WithModel("gemini-2.5-flash"),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.GenerateContent(context.Background(), []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Hello"),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Choices[0].Content)
}
