package email

import (
	"context"
	"strings"

	mw "iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
)

func init() {
	mw.Register(EmailTools{})
	mw.Register(EmailExec{})
}

// EmailTools injects email search/send tools.
type EmailTools struct{}

func (EmailTools) ID() string    { return "email_tools" }
func (EmailTools) Priority() int { return 85 }

func (EmailTools) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["email_tools"].(bool); ok {
		return v
	}
	return true
}

func (EmailTools) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	tools := []llms.Tool{
		funcTool("search_emails", "Search emails by query", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "search query (e.g., from:alice subject:invoice)"},
				"limit": map[string]any{"type": "integer", "description": "max results (default 5)"},
			},
			"required": []string{"query"},
		}),
		funcTool("send_email", "Send an email", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"to":      map[string]any{"type": "string", "description": "recipient email"},
				"subject": map[string]any{"type": "string", "description": "email subject"},
				"body":    map[string]any{"type": "string", "description": "email body content"},
			},
			"required": []string{"to", "subject", "body"},
		}),
	}
	params.Tools = append(params.Tools, tools...)
	if params.ToolChoice == nil {
		params.ToolChoice = "auto"
	}

	return mw.Decision{
		OverrideParams: params,
		Reason:         "email_tools: injected tools",
	}, nil
}

// EmailExec executes email tools.
type EmailExec struct{}

func (EmailExec) ID() string    { return "email_exec" }
func (EmailExec) Priority() int { return 80 }

func (EmailExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (EmailExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
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
		case "search_emails":
			query, _ := tc.Args["query"].(string)
			out := searchEmails(query)
			outputs = append(outputs, out)
		case "send_email":
			to, _ := tc.Args["to"].(string)
			subj, _ := tc.Args["subject"].(string)
			body, _ := tc.Args["body"].(string)
			out := sendEmail(to, subj, body)
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
		Reason:      "email_exec",
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
