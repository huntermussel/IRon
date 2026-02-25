package llm

import (
	"context"
	"fmt"
	"iron/internal/chat"
	"iron/internal/middleware"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type OpenAIAdapter struct {
	client *openai.LLM
	model  string
}

func NewOpenAIAdapter(model, baseURL string) (chat.Adapter, error) {
	opts := []openai.Option{
		openai.WithModel(model),
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	if token := os.Getenv("OPENAI_API_KEY"); token != "" {
		opts = append(opts, openai.WithToken(token))
	}

	client, err := openai.New(opts...)
	if err != nil {
		return nil, err
	}
	return &OpenAIAdapter{client: client, model: model}, nil
}

func (a *OpenAIAdapter) ReplyStream(ctx context.Context, history []chat.Message, params *middleware.LLMParams, streamFn func(string)) (string, []chat.ToolCall, error) {
	// Reusing logic similar to OllamaAdapter but specific to OpenAI struct
	messages := make([]llms.MessageContent, 0, len(history))
	for _, m := range history {
		switch m.Role {
		case chat.RoleUser:
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, m.Content))
		case chat.RoleAssistant:
			var parts []llms.ContentPart
			if m.Content != "" {
				parts = append(parts, llms.TextPart(m.Content))
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, llms.ToolCall{
					ID:   tc.ID,
					Type: "function",
					FunctionCall: &llms.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			if len(parts) == 0 {
				parts = append(parts, llms.TextPart(" "))
			}
			messages = append(messages, llms.MessageContent{
				Role:  llms.ChatMessageTypeAI,
				Parts: parts,
			})
		case chat.RoleSystem:
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, m.Content))
		case chat.RoleTool:
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: m.ToolCallID,
						Content:    m.Content,
					},
				},
			})
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
		if len(params.Tools) > 0 {
			opts = append(opts, llms.WithTools(params.Tools))
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

	var toolCalls []chat.ToolCall
	for _, tc := range resp.Choices[0].ToolCalls {
		toolCalls = append(toolCalls, chat.ToolCall{
			ID:        tc.ID,
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		})
	}

	return resp.Choices[0].Content, toolCalls, nil
}
