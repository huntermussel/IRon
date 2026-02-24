package handlers

import (
	"fmt"
	"regexp"
	"strings"

	"iron/middlewares/nlurouter/nlp"
)

// WeatherHandler
type WeatherHandler struct{}

var reWeather = regexp.MustCompile(`weather in (?P<location>.+)`)

func (h *WeatherHandler) Match(input string, tokens []nlp.Token) (bool, map[string]string, float64) {
	if strings.Contains(input, "weather") {
		matches := reWeather.FindStringSubmatch(input)
		if len(matches) > 1 {
			return true, map[string]string{"location": strings.TrimSpace(matches[1])}, 1.0
		}
		return true, nil, 0.6 // Keyword match but no slot -> Complex
	}
	return false, nil, 0.0
}

func (h *WeatherHandler) Handle(slots map[string]string) (any, error) {
	loc := slots["location"]
	// Mock logic: deterministic data response
	return map[string]any{
		"location":    loc,
		"condition":   "Sunny",
		"temperature": 25,
		"unit":        "C",
		"message":     fmt.Sprintf("The weather in %s is Sunny, 25°C.", loc),
	}, nil
}

func (h *WeatherHandler) SystemPrompt() string {
	return "You are a weather assistant. The user is asking about weather. Be precise and concise."
}
