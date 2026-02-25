package llm

import (
	"context"
	"fmt"
	"os"

	"iron/internal/chat"
	"iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

type GeminiAdapter struct {
	client *googleai.GoogleAI
	model  string
}

func NewGeminiAdapter(model, baseURL string) (chat.Adapter, error) {
	effectiveModel := model
	if effectiveModel == "" {
		effectiveModel = googleai.DefaultOptions().DefaultModel
	}

	opts := []googleai.Option{
		googleai.WithDefaultModel(effectiveModel),
	}
	if baseURL != "" {
		opts = append(opts, googleai.WithRest())
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		opts = append(opts, googleai.WithAPIKey(key))
	}

	ctx := context.Background()
	client, err := googleai.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &GeminiAdapter{
		client: client,
		model:  effectiveModel,
	}, nil
}

func (a *GeminiAdapter) ReplyStream(ctx context.Context, history []chat.Message, params *middleware.LLMParams, streamFn func(string)) (string, []chat.ToolCall, error) {
	messages := convertHistory(history)

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
		return "", nil, fmt.Errorf("empty response from Gemini model")
	}

	choice := resp.Choices[0]
	toolCalls := make([]chat.ToolCall, 0, len(choice.ToolCalls))
	for _, tc := range choice.ToolCalls {
		if tc.FunctionCall == nil {
			continue
		}
		toolCalls = append(toolCalls, chat.ToolCall{
			ID:        tc.ID,
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		})
	}

	return choice.Content, toolCalls, nil
}

func convertHistory(history []chat.Message) []llms.MessageContent {
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
	return messages
}
