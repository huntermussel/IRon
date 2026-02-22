package llm

import (
	"context"
	"fmt"
	"iron/internal/chat"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

type OllamaAdapter struct {
	client *ollama.LLM
	model  string
}

func NewOllamaAdapter(model, baseURL string) (chat.Adapter, error) {
	var opts []ollama.Option

	if baseURL != "" {
		opts = append(opts, ollama.WithServerURL(baseURL))
	}
	client, err := ollama.New(opts...)
	if err != nil {
		return nil, err
	}
	return &OllamaAdapter{client: client, model: model}, nil
}

func (a *OllamaAdapter) Reply(ctx context.Context, history []chat.Message) (string, error) {
	messages := make([]llms.MessageContent, 0, len(history))
	for _, m := range history {
		switch m.Role {
		case chat.RoleUser:
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, m.Content))
		case chat.RoleAssistant:
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeAI, m.Content))
		}
	}

	resp, err := a.client.GenerateContent(ctx, messages, llms.WithModel(a.model))
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from model")
	}
	return resp.Choices[0].Content, nil
}
