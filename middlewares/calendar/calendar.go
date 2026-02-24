package calendar

import (
	"context"
	"strings"

	mw "iron/internal/middleware"

	"github.com/tmc/langchaingo/llms"
)

func init() {
	mw.Register(CalendarTools{})
	mw.Register(CalendarExec{})
}

// CalendarTools injects calendar listing/creation tools.
type CalendarTools struct{}

func (CalendarTools) ID() string    { return "calendar_tools" }
func (CalendarTools) Priority() int { return 85 }

func (CalendarTools) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["calendar_tools"].(bool); ok {
		return v
	}
	return true
}

func (CalendarTools) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	tools := []llms.Tool{
		funcTool("list_calendar_events", "List calendar events for a date range", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start_date": map[string]any{"type": "string", "description": "start date (YYYY-MM-DD)"},
				"end_date":   map[string]any{"type": "string", "description": "end date (YYYY-MM-DD)"},
				"limit":      map[string]any{"type": "integer", "description": "max results (default 5)"},
			},
			"required": []string{"start_date"},
		}),
		funcTool("create_calendar_event", "Create a calendar event", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":       map[string]any{"type": "string", "description": "event title"},
				"start_time":  map[string]any{"type": "string", "description": "start time (RFC3339)"},
				"end_time":    map[string]any{"type": "string", "description": "end time (RFC3339)"},
				"description": map[string]any{"type": "string", "description": "event description"},
			},
			"required": []string{"title", "start_time", "end_time"},
		}),
	}
	params.Tools = append(params.Tools, tools...)
	if params.ToolChoice == nil {
		params.ToolChoice = "auto"
	}

	return mw.Decision{
		OverrideParams: params,
		Reason:         "calendar_tools: injected tools",
	}, nil
}

// CalendarExec executes calendar tools.
type CalendarExec struct{}

func (CalendarExec) ID() string    { return "calendar_exec" }
func (CalendarExec) Priority() int { return 80 }

func (CalendarExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (CalendarExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
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
		case "list_calendar_events":
			start, _ := tc.Args["start_date"].(string)
			out := listCalendarEvents(start)
			outputs = append(outputs, out)
		case "create_calendar_event":
			title, _ := tc.Args["title"].(string)
			start, _ := tc.Args["start_time"].(string)
			end, _ := tc.Args["end_time"].(string)
			out := createCalendarEvent(title, start, end)
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
		Reason:      "calendar_exec",
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
