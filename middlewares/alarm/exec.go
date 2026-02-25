package alarm

import (
	"context"
	"fmt"
	"strings"

	mw "iron/internal/middleware"
)

// AlarmExec executes alarm.set tool calls emitted by the model.
// It runs on after_llm_response and cancels further processing when it handles
// at least one alarm tool call.
type AlarmExec struct{}

func (AlarmExec) ID() string    { return "alarm_exec" }
func (AlarmExec) Priority() int { return 70 }

func (AlarmExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (AlarmExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventAfterLLMResponse {
		return mw.Decision{}, nil
	}

	raw, ok := e.Context["tool_calls"].([]mw.ToolCall)
	if !ok || len(raw) == 0 {
		return mw.Decision{}, nil
	}

	outputs := make([]string, 0, len(raw))
	for _, tc := range raw {
		if tc.Tool != "alarm.set" {
			continue
		}
		out := runAlarmTool(tc)
		if strings.TrimSpace(out) != "" {
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
		Reason:      "alarm_exec",
	}, nil
}

func runAlarmTool(tc mw.ToolCall) string {
	timeStr, _ := tc.Args["time"].(string)
	label, _ := tc.Args["label"].(string)

	if strings.TrimSpace(timeStr) == "" {
		return `alarm.set: missing required arg "time"`
	}

	if strings.TrimSpace(label) == "" {
		return fmt.Sprintf("ok: alarm set for %s", timeStr)
	}
	return fmt.Sprintf("ok: alarm set for %s (%s)", timeStr, label)
}
