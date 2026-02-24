package handlers

import (
	"fmt"
	"regexp"
	"strings"

	"iron/middlewares/nlurouter/nlp"
)

// AlarmHandler
type AlarmHandler struct{}

var reAlarm = regexp.MustCompile(`set (?:an )?alarm for (?P<time>.+)`)

func (h *AlarmHandler) Match(input string, tokens []nlp.Token) (bool, map[string]string, float64) {
	if strings.Contains(input, "alarm") {
		matches := reAlarm.FindStringSubmatch(input)
		if len(matches) > 1 {
			return true, map[string]string{"time": strings.TrimSpace(matches[1])}, 1.0
		}
		return true, nil, 0.6
	}
	return false, nil, 0.0
}

func (h *AlarmHandler) Handle(slots map[string]string) (any, error) {
	t := slots["time"]
	return map[string]any{
		"action":  "set_alarm",
		"time":    t,
		"status":  "success",
		"message": fmt.Sprintf("Alarm set for %s.", t),
	}, nil
}

func (h *AlarmHandler) SystemPrompt() string {
	return ""
}
