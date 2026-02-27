package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" { key = os.Getenv("GOOGLE_API_KEY") }
	if key == "" { key = os.Getenv("IRON_GEMINI_API_KEY") }

	client, err := genai.NewClient(ctx, option.WithAPIKey(key))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	iter := client.ListModels(ctx)
	for {
		m, err := iter.Next()
		if err != nil {
			break
		}
		fmt.Println(m.Name)
	}
}
