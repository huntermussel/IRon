package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"iron/internal/chat"
	"iron/internal/middleware"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type OllamaAdapter struct {
	client *openai.LLM
	model  string
}

func NewOllamaAdapter(model, baseURL string) (chat.Adapter, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	} else if !strings.HasSuffix(baseURL, "/v1") && !strings.HasSuffix(baseURL, "/v1/") {
		baseURL = strings.TrimSuffix(baseURL, "/") + "/v1"
	}

	opts := []openai.Option{
		openai.WithBaseURL(baseURL),
		openai.WithToken("ollama"), // dummy token
		openai.WithModel(model),
	}

	client, err := openai.New(opts...)
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
			// Reconstruct Assistant message with ToolCalls if present
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
			// If both empty, add a space to avoid error
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
			// Tool results must be linked to a ToolCall ID.
			// `langchaingo` generic message content supports `ChatMessageTypeTool`.
			// The content part should be a ToolResultPart or similar if available,
			// but generic interface uses `TextParts`.
			// However, `llms.MessageContent` has a `Parts` slice.
			// We can construct a `ToolCallResponse` part if supported, or just text with ID if the provider supports it.
			// For Ollama/OpenAI via langchaingo, we usually need to set the ToolCallID on the message itself?
			// Checking `llms` package: `MessageContent` doesn't have ID field directly on struct,
			// but `ToolCallResponse` part does.

			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: m.ToolCallID,
						Name:       m.ToolName,
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
			opts = append(opts, llms.WithToolChoice("auto"))
		}
	}

	// Disable streaming if tools are present to ensure stable tool calling output
	if params == nil || len(params.Tools) == 0 {
		opts = append(opts, llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
			if streamFn != nil {
				streamFn(string(chunk))
			}
			return nil
		}))
	}

	resp, err := a.client.GenerateContent(ctx, messages, opts...)
	if err != nil {
		return "", nil, err
	}
	if len(resp.Choices) == 0 {
		return "", nil, fmt.Errorf("empty response from model")
	}

	// Extract tool calls
	var toolCalls []chat.ToolCall
	for _, tc := range resp.Choices[0].ToolCalls {
		toolCalls = append(toolCalls, chat.ToolCall{
			ID:        tc.ID,
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		})
	}

	// Heuristic fallback for models that output JSON in text instead of ToolCalls field
	if len(toolCalls) == 0 && resp.Choices[0].Content != "" {
		content := strings.TrimSpace(resp.Choices[0].Content)
		// Remove markdown code blocks if present
		if strings.HasPrefix(content, "```") {
			content = strings.TrimPrefix(content, "```json")
			content = strings.TrimPrefix(content, "```")
			content = strings.TrimSuffix(content, "```")
			content = strings.TrimSpace(content)
		}

		if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
			var h struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
				Params    map[string]any `json:"parameters"`
			}
			if err := json.Unmarshal([]byte(content), &h); err == nil && h.Name != "" {
				args := h.Arguments
				if args == nil {
					args = h.Params
				}
				if args == nil {
					args = make(map[string]any)
				}
				argsBytes, _ := json.Marshal(args)
				toolCalls = append(toolCalls, chat.ToolCall{
					ID:        fmt.Sprintf("h_%d", time.Now().UnixNano()),
					Name:      h.Name,
					Arguments: string(argsBytes),
				})
				// If we found a tool call, we assume the content was just the tool call
				return "", toolCalls, nil
			}
		}
	}

	return resp.Choices[0].Content, toolCalls, nil
}
