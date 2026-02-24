package handlers

import (
	"fmt"
	"regexp"
	"strings"

	"iron/middlewares/nlurouter/nlp"
)

// DeviceHandler
type DeviceHandler struct{}

var reDevice = regexp.MustCompile(`turn (?P<state>on|off) (?:the )?(?P<device>.+)`)

func (h *DeviceHandler) Match(input string, tokens []nlp.Token) (bool, map[string]string, float64) {
	if strings.Contains(input, "turn") {
		matches := reDevice.FindStringSubmatch(input)
		if len(matches) > 2 {
			return true, map[string]string{"state": matches[1], "device": strings.TrimSpace(matches[2])}, 1.0
		}
		return true, nil, 0.5
	}
	return false, nil, 0.0
}

func (h *DeviceHandler) Handle(slots map[string]string) (any, error) {
	return map[string]any{
		"action":  "control_device",
		"device":  slots["device"],
		"state":   slots["state"],
		"status":  "success",
		"message": fmt.Sprintf("Turned %s the %s.", slots["state"], slots["device"]),
	}, nil
}

func (h *DeviceHandler) SystemPrompt() string {
	return "You are a smart home assistant. Control devices."
}
