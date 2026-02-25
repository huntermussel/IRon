package skills

import (
	"context"
	"fmt"
	"iron/internal/browser"
)

// BrowserSkill allows the agent to browse the web.
type BrowserSkill struct {
	Controller *browser.Controller
}

func (b *BrowserSkill) Name() string { return "browser" }
func (b *BrowserSkill) Description() string {
	return "Browses a URL and extracts text content. Use this to read web pages."
}
func (b *BrowserSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"read", "screenshot"},
				"description": "Action to perform.",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to visit.",
			},
		},
		"required": []string{"action", "url"},
	}
}

func (b *BrowserSkill) Execute(ctx context.Context, args map[string]any) (string, error) {
	if b.Controller == nil {
		return "", fmt.Errorf("browser controller not initialized")
	}

	action, _ := args["action"].(string)
	url, _ := args["url"].(string)

	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	switch action {
	case "read":
		text, err := b.Controller.NavigateAndExtract(url)
		if err != nil {
			return "", err
		}
		if len(text) > 8000 {
			text = text[:8000] + "\n...(truncated)"
		}
		return text, nil

	case "screenshot":
		data, err := b.Controller.Screenshot(url)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Screenshot taken (%d bytes). (Display not supported in CLI)", len(data)), nil

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}
