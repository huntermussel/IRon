package skills

import (
	"context"
	"fmt"
	"iron/internal/memory"
)

// MemorySkill allows the agent to store and retrieve facts.
type MemorySkill struct {
	Store *memory.Store
}

func (m *MemorySkill) Name() string { return "memory" }
func (m *MemorySkill) Description() string {
	return "Stores or retrieves information from long-term memory."
}
func (m *MemorySkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"remember", "recall"},
				"description": "Action to perform.",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "The key or topic (for remember).",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "The information to store (for remember).",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "The search query (for recall).",
			},
		},
		"required": []string{"action"},
	}
}

func (m *MemorySkill) Execute(ctx context.Context, args map[string]any) (string, error) {
	if m.Store == nil {
		return "", fmt.Errorf("memory store not initialized")
	}

	action, _ := args["action"].(string)
	switch action {
	case "remember":
		key, _ := args["key"].(string)
		value, _ := args["value"].(string)
		if value == "" {
			return "", fmt.Errorf("value is required for remember")
		}
		text := value
		if key != "" {
			text = fmt.Sprintf("%s: %s", key, value)
		}
		m.Store.Index("user_facts", text)
		return fmt.Sprintf("Remembered: %s", text), nil

	case "recall":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query is required for recall")
		}
		hits := m.Store.Query("user_facts", query, 5)
		if len(hits) == 0 {
			return "No relevant memories found.", nil
		}
		// Formatting list
		var result string
		for _, h := range hits {
			result += "- " + h + "\n"
		}
		return result, nil

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}
