package middleware

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

type EventName string

const (
	EventBeforeLLMRequest EventName = "before_llm_request"
	EventAfterLLMResponse EventName = "after_llm_response"
	EventBeforeUserReply  EventName = "before_user_reply"
)

type LLMParams struct {
	Model            string
	Messages         []Message // chat history
	Temperature      float64
	TopP             float64
	MaxTokens        int
	FrequencyPenalty float64
	PresencePenalty  float64
	Stop             []string
	Seed             *int

	// Tool / function calling schema (LangChainGo).
	//
	// Tools is the preferred mechanism. Functions are supported for backends that
	// still accept them but are deprecated upstream.
	Tools      []llms.Tool               // llms.WithTools(...)
	ToolChoice any                       // "none" | "auto" | llms.ToolChoice
	Functions  []llms.FunctionDefinition // llms.WithFunctions(...) (deprecated)
}

type Message struct {
	Role string // "system"|"user"|"assistant"|"tool"
	Text string
}

type ToolCall struct {
	Tool string
	Args map[string]any
}

type Decision struct {
	Cancel      bool   // stop the pipeline for this event
	Reprompt    bool   // core should call LLM again
	Reason      string // for logs
	ReplaceText *string

	// Optional: change request + continue
	OverrideParams *LLMParams

	// Optional: request tools (core executes)
	ToolCalls []ToolCall
}

type Event struct {
	Name     EventName
	UserText string     // for before_llm_request
	LLMText  string     // for after_llm_response
	Params   *LLMParams // mutable
	Attempt  int        // retry attempt number
	MaxRetry int
	Context  map[string]any // workspace, open file, mode, etc.
}

type Middleware interface {
	ID() string
	Priority() int
	OnEvent(ctx context.Context, e *Event) (Decision, error)
}

// ConditionalMiddleware is an optional extension that allows a middleware to be
// dynamically enabled/disabled per request/event.
//
// If a middleware implements this interface and returns false, it will be
// skipped during dispatch (but still recorded in results with a "skipped"
// reason).
type ConditionalMiddleware interface {
	ShouldLoad(ctx context.Context, e *Event) bool
}
