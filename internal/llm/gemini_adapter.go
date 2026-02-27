package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"iron/internal/chat"
	"iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type GeminiAdapter struct {
	client *openai.LLM
	model  string
}

func NewGeminiAdapter(model, baseURL string) (chat.Adapter, error) {
	effectiveModel := model
	if effectiveModel == "" {
		effectiveModel = "gemini-2.5-flash"
	}

	// Remove "models/" prefix if it was passed, because the OpenAI endpoint doesn't want it
	effectiveModel = strings.TrimPrefix(effectiveModel, "models/")
	effectiveModel = strings.TrimPrefix(effectiveModel, "tunedModels/")

	apiKey := os.Getenv("IRON_GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GENAI_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key not found. Please set IRON_GEMINI_API_KEY, GEMINI_API_KEY, or GOOGLE_API_KEY")
	}

	// Use Google's official OpenAI-compatible endpoint
	endpoint := "https://generativelanguage.googleapis.com/v1beta/openai/"
	if baseURL != "" && strings.Contains(baseURL, "googleapis.com") {
		endpoint = baseURL
	} else if baseURL != "" {
		fmt.Printf("Warning: Custom baseURL provided for Gemini but using OpenAI compatibility layer: %s\n", baseURL)
		endpoint = baseURL
	}

	opts := []openai.Option{
		openai.WithBaseURL(endpoint),
		openai.WithToken(apiKey),
		openai.WithModel(effectiveModel),
	}

	client, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Gemini (OpenAI-compatible) client: %w", err)
	}

	return &GeminiAdapter{
		client: client,
		model:  effectiveModel,
	}, nil
}

func (a *GeminiAdapter) ReplyStream(ctx context.Context, history []chat.Message, params *middleware.LLMParams, streamFn func(string)) (string, []chat.ToolCall, error) {
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
			m := strings.TrimPrefix(params.Model, "models/")
			opts = append(opts, llms.WithModel(m))
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

	// Disable streaming if tools are present to ensure stable tool calling output parsing
	numTools := 0
	if params != nil {
		numTools = len(params.Tools)
	}
	fmt.Printf("DEBUG [GeminiAdapter]: Proceeding to GenerateContent with %d tools\n", numTools)

	// Disable streaming if tools are present to ensure stable tool calling output parsing
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

	choice := resp.Choices[0]
	fmt.Printf("DEBUG: Gemini raw response choice: %+v\n", choice)

	// Emulate streaming output for non-streaming tool calls so UI sees it
	if streamFn != nil && choice.Content != "" && (params != nil && len(params.Tools) > 0) {
		streamFn(choice.Content)
	}

	var toolCalls []chat.ToolCall
	for _, tc := range choice.ToolCalls {
		toolCalls = append(toolCalls, chat.ToolCall{
			ID:        tc.ID,
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		})
	}

	return choice.Content, toolCalls, nil
}
