package weather

import (
	"context"
	"encoding/json"
	"fmt"

	"iron/internal/middleware"
	"iron/internal/nlu"
)

// WeatherMiddleware handles weather requests.
type WeatherMiddleware struct {
	nluEngine *nlu.Engine
}

func init() {
	middleware.Register(New())
}

func New() *WeatherMiddleware {
	engine := nlu.GetEngine()
	// Register structured utterances for this intent
	engine.RegisterIntent("get_weather",
		"weather in {location}",
		"what is the weather in {location}",
		"forecast for {location}",
		"temperature in {location}",
	)

	return &WeatherMiddleware{
		nluEngine: engine,
	}
}

func (m *WeatherMiddleware) ID() string {
	return "weather"
}

func (m *WeatherMiddleware) Priority() int {
	return 100
}

// ShouldLoad checks if the input matches the weather intent using the shared NLU engine.
func (m *WeatherMiddleware) ShouldLoad(ctx context.Context, e *middleware.Event) bool {
	if e.Name != middleware.EventBeforeLLMRequest {
		return false
	}

	result := m.nluEngine.Parse(e.UserText)
	return result.Intent == "get_weather"
}

func (m *WeatherMiddleware) OnEvent(ctx context.Context, e *middleware.Event) (middleware.Decision, error) {
	result := m.nluEngine.Parse(e.UserText)

	loc := "unknown location"
	if l, ok := result.Slots["location"]; ok {
		loc = l
	} else {
		// Fallback for partial matches or complex queries handled by LLM?
		// For now, simple deterministic response.
		// If location is missing, we might want to ask LLM, but here we enforce local handling.
		loc = "local area"
	}

	// Mock logic: deterministic data response
	respData := map[string]any{
		"location":    loc,
		"condition":   "Sunny",
		"temperature": 25,
		"unit":        "C",
		"message":     fmt.Sprintf("The weather in %s is Sunny, 25°C.", loc),
	}

	jsonBytes, err := json.Marshal(respData)
	if err != nil {
		return middleware.Decision{}, err
	}

	respStr := string(jsonBytes)
	return middleware.Decision{
		Cancel:      true,
		ReplaceText: &respStr,
		Reason:      "weather: handled locally via NLU match",
	}, nil
}
