package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
		"{action} every {duration}",         // e.g. "search every hour"
		"{action} {count} times per {unit}", // e.g. "search 1 time per week"
		"{action} {count} time per {unit}",
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

	// Handle complex duration patterns
	if d, ok := result.Slots["duration"]; ok {
		duration = d
	} else if u, ok := result.Slots["unit"]; ok {
		// Reconstruct duration from count/unit
		c := "1"
		if count, ok := result.Slots["count"]; ok {
			c = count
		}
		duration = fmt.Sprintf("%s times per %s", c, u)
	}

	// Heuristic to generate cron expression
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
func parseDurationToCron(duration string) string {
	duration = strings.ToLower(duration)

	// Handle explicit "times per" pattern
	if strings.Contains(duration, "times per") || strings.Contains(duration, "time per") {
		parts := strings.Fields(duration)
		// e.g. "1 time per week"
		if len(parts) >= 4 {
			countStr := parts[0]
			unit := parts[3] // "week", "day", etc.

			// Simple frequency handling
			if strings.Contains(unit, "week") {
				return "0 0 * * 0" // Weekly (Sunday)
			}
			if strings.Contains(unit, "day") {
				// "2 times per day" -> every 12 hours
				count, _ := strconv.Atoi(countStr)
				if count > 0 {
					hours := 24 / count
					if hours > 0 {
						return fmt.Sprintf("0 */%d * * *", hours)
					}
					return "0 * * * *" // fallback hourly
				}
				return "0 0 * * *" // default daily
			}
			if strings.Contains(unit, "hour") {
				// "2 times per hour" -> every 30 mins
				count, _ := strconv.Atoi(countStr)
				if count > 0 {
					mins := 60 / count
					if mins > 0 {
						return fmt.Sprintf("*/%d * * * *", mins)
					}
				}
				return "0 * * * *" // default hourly
			}
		}
	}

	// Handle "every X hours"
	if strings.Contains(duration, "hour") {
		parts := strings.Fields(duration)
		if len(parts) > 0 {
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

	// Handle "every week", "weekly"
	if strings.Contains(duration, "week") {
		return "0 0 * * 0" // Sunday midnight
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
