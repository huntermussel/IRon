package llm

import (
	"context"
	"fmt"
	"iron/internal/chat"
	"iron/internal/middleware"

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

func (a *OllamaAdapter) ReplyStream(ctx context.Context, history []chat.Message, params *middleware.LLMParams, streamFn func(string)) (string, []chat.ToolCall, error) {
	messages := make([]llms.MessageContent, 0, len(history))
	for _, m := range history {
		switch m.Role {
		case chat.RoleUser:
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, m.Content))
		case chat.RoleAssistant:
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeAI, m.Content))
		}
	}

	opts := make([]llms.CallOption, 0, 8)
	opts = append(opts, llms.WithModel(a.model))
	if params != nil {
		if params.Model != "" {
			opts = append(opts, llms.WithModel(params.Model))
		}
		if params.Temperature != 0 {
			opts = append(opts, llms.WithTemperature(params.Temperature))
		}
		if params.TopP != 0 {
			opts = append(opts, llms.WithTopP(params.TopP))
		}
		if params.MaxTokens != 0 {
			opts = append(opts, llms.WithMaxTokens(params.MaxTokens))
		}
		if params.FrequencyPenalty != 0 {
			opts = append(opts, llms.WithFrequencyPenalty(params.FrequencyPenalty))
		}
		if params.PresencePenalty != 0 {
			opts = append(opts, llms.WithPresencePenalty(params.PresencePenalty))
		}
		if len(params.Stop) > 0 {
			opts = append(opts, llms.WithStopWords(params.Stop))
		}
		if params.Seed != nil {
			opts = append(opts, llms.WithSeed(*params.Seed))
		}
		if len(params.Tools) > 0 {
			opts = append(opts, llms.WithTools(params.Tools))
		}
		if params.ToolChoice != nil {
			opts = append(opts, llms.WithToolChoice(params.ToolChoice))
		}
		if len(params.Functions) > 0 {
			opts = append(opts, llms.WithFunctions(params.Functions))
		}
	}
	opts = append(opts, llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
		if streamFn != nil {
			streamFn(string(chunk))
		}
		return nil
	}))

	resp, err := a.client.GenerateContent(ctx, messages, opts...)
	if err != nil {
		return "", nil, err
	}
	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("empty response from model")
	}
	return resp.Choices[0].Content, nil, nil
}
