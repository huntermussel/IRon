package chat

import (
	"context"

	"iron/internal/middleware"
)

// Adapter abstracts chat completion providers.
type Adapter interface {
	// ReplyStream should stream assistant text chunks to streamFn (if non-nil)
	// and return the full text plus any tool calls the model emitted.
	ReplyStream(ctx context.Context, history []Message, params *middleware.LLMParams, streamFn func(string)) (text string, toolCalls []ToolCall, err error)
}

// ToolCall mirrors llms.ToolCall but keeps adapter decoupled.
type ToolCall struct {
	Name      string
	Arguments string
}
