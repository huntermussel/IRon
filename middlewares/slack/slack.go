package slack

import (
	"context"
	"strings"

	mw "iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
)

func init() {
	mw.Register(SlackTools{})
	mw.Register(SlackExec{})
}

// SlackTools injects slack read/send tools.
type SlackTools struct{}

func (SlackTools) ID() string    { return "slack_tools" }
func (SlackTools) Priority() int { return 85 }

func (SlackTools) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["slack_tools"].(bool); ok {
		return v
	}
	return true
}

func (SlackTools) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	tools := []llms.Tool{
		funcTool("read_slack_channel", "Read messages from a Slack channel", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"channel": map[string]any{"type": "string", "description": "channel name (e.g., #general)"},
				"limit":   map[string]any{"type": "integer", "description": "max messages (default 10)"},
			},
			"required": []string{"channel"},
		}),
		funcTool("send_slack_message", "Send a message to a Slack channel", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"channel": map[string]any{"type": "string", "description": "channel name (e.g., #general)"},
				"text":    map[string]any{"type": "string", "description": "message text"},
			},
			"required": []string{"channel", "text"},
		}),
	}
	params.Tools = append(params.Tools, tools...)
	if params.ToolChoice == nil {
		params.ToolChoice = "auto"
	}

	return mw.Decision{
		OverrideParams: params,
		Reason:         "slack_tools: injected tools",
	}, nil
}

// SlackExec executes slack tools.
type SlackExec struct{}

func (SlackExec) ID() string    { return "slack_exec" }
func (SlackExec) Priority() int { return 80 }

func (SlackExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (SlackExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventAfterLLMResponse {
		return mw.Decision{}, nil
	}
	raw, ok := e.Context["tool_calls"].([]mw.ToolCall)
	if !ok || len(raw) == 0 {
		return mw.Decision{}, nil
	}

	outputs := make([]string, 0, len(raw))
	for _, tc := range raw {
		switch tc.Tool {
		case "read_slack_channel":
			channel, _ := tc.Args["channel"].(string)
			out := readSlackChannel(channel)
			outputs = append(outputs, out)
		case "send_slack_message":
			channel, _ := tc.Args["channel"].(string)
			text, _ := tc.Args["text"].(string)
			out := sendSlackMessage(channel, text)
			outputs = append(outputs, out)
		}
	}
	if len(outputs) == 0 {
		return mw.Decision{}, nil
	}
	text := strings.Join(outputs, "\n\n")
	return mw.Decision{
		Cancel:      true,
		ReplaceText: &text,
		Reason:      "slack_exec",
	}, nil
}

func funcTool(name, desc string, params any) llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: desc,
			Parameters:  params,
		},
	}
}
