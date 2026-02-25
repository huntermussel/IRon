package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"iron/internal/middleware"
	"iron/internal/nlu"
)

// CronMiddleware handles recurring task scheduling.
type CronMiddleware struct {
	nluEngine *nlu.Engine
}

func init() {
	middleware.Register(New())
}

func New() *CronMiddleware {
	engine := nlu.GetEngine()
	// Register structured utterances for this intent
	engine.RegisterIntent("set_cron",
		"remind me to {action} every {duration}",
		"schedule {action} every {duration}",
		"run {action} every {duration}",
		"execute {action} every {duration}",
		"do {action} every {duration}",
	)

	return &CronMiddleware{
		nluEngine: engine,
	}
}

func (m *CronMiddleware) ID() string {
	return "cron"
}

func (m *CronMiddleware) Priority() int {
	return 100
}

// ShouldLoad checks if the input matches the cron intent using the shared NLU engine.
func (m *CronMiddleware) ShouldLoad(ctx context.Context, e *middleware.Event) bool {
	if e.Name != middleware.EventBeforeLLMRequest {
		return false
	}

	result := m.nluEngine.Parse(e.UserText)
	return result.Intent == "set_cron"
}

func (m *CronMiddleware) OnEvent(ctx context.Context, e *middleware.Event) (middleware.Decision, error) {
	result := m.nluEngine.Parse(e.UserText)

	action := "unknown action"
	duration := "unknown duration"

	if a, ok := result.Slots["action"]; ok {
		action = a
	}
	if d, ok := result.Slots["duration"]; ok {
		duration = d
	}

	// Simple heuristic to generate cron expression
	cronExpr := parseDurationToCron(duration)

	respData := map[string]any{
		"action":      "set_cron",
		"task":        action,
		"duration":    duration,
		"cron_expr":   cronExpr,
		"status":      "success",
		"message":     fmt.Sprintf("Scheduled '%s' every %s (Cron: %s).", action, duration, cronExpr),
		"instruction": fmt.Sprintf("To install, run: (crontab -l 2>/dev/null; echo \"%s notify-send '%s'\") | crontab -", cronExpr, action),
	}

	jsonBytes, err := json.Marshal(respData)
	if err != nil {
		return middleware.Decision{}, err
	}

	respStr := string(jsonBytes)
	return middleware.Decision{
		Cancel:      true,
		ReplaceText: &respStr,
		Reason:      "cron: handled locally via NLU match",
	}, nil
}

// parseDurationToCron converts natural language duration to basic cron syntax.
// This is a naive implementation for demonstration.
func parseDurationToCron(duration string) string {
	duration = strings.ToLower(duration)

	// Handle "every X hours"
	if strings.Contains(duration, "hour") {
		parts := strings.Fields(duration)
		if len(parts) > 0 {
			// Extract number if present (e.g. "2 hours")
			// Naive check: if first word is a number
			if isDigit(parts[0]) {
				return fmt.Sprintf("0 */%s * * *", parts[0])
			}
			// "every hour" -> "0 * * * *"
			return "0 * * * *"
		}
	}

	// Handle "every X minutes"
	if strings.Contains(duration, "minute") {
		parts := strings.Fields(duration)
		if len(parts) > 0 {
			if isDigit(parts[0]) {
				return fmt.Sprintf("*/%s * * * *", parts[0])
			}
			return "* * * * *"
		}
	}

	// Default daily
	if strings.Contains(duration, "day") {
		return "0 0 * * *"
	}

	return "* * * * *" // Fallback: every minute
}

func isDigit(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
