package alarm

import (
	"strings"

	"github.com/tmc/langchaingo/llms"
)

func upsertTool(tools []llms.Tool, t llms.Tool) []llms.Tool {
	name := ""
	if t.Function != nil {
		name = strings.TrimSpace(t.Function.Name)
	}
	if name == "" {
		return append(tools, t)
	}
	for _, existing := range tools {
		if existing.Function == nil {
			continue
		}
		if strings.TrimSpace(existing.Function.Name) == name {
			return tools
		}
	}
	return append(tools, t)
}

func AlarmSetTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "alarm.set",
			Description: "Set an alarm at a given time",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"time": map[string]any{
						"type":        "string",
						"description": "Time for the alarm (e.g. 07:30, 7am, tomorrow 8)",
					},
					"label": map[string]any{
						"type":        "string",
						"description": "Optional description",
					},
				},
				"required": []string{"time"},
			},
		},
	}
}

func TimerSetTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "timer.set",
			Description: "Set a timer or reminder that will notify you after a specified duration.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"minutes": map[string]any{
						"type":        "number",
						"description": "Duration in minutes until the timer goes off",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "The reminder message to display",
					},
				},
				"required": []string{"minutes", "message"},
			},
		},
	}
}
