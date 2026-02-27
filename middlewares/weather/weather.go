package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"iron/internal/middleware"
	"iron/internal/nlu"

	"github.com/tmc/langchaingo/llms"
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
	return 110
}

// ShouldLoad checks if the input matches the weather intent or contains tool calls.
func (m *WeatherMiddleware) ShouldLoad(ctx context.Context, e *middleware.Event) bool {
	if e.Name == middleware.EventBeforeLLMRequest {
		return true
	}
	if e.Name == middleware.EventAfterLLMResponse {
		if e.Context == nil {
			return false
		}
		calls, ok := e.Context["tool_calls"].([]middleware.ToolCall)
		if !ok {
			return false
		}
		for _, tc := range calls {
			if tc.Tool == "get_weather" {
				return true
			}
		}
	}
	return false
}

func (m *WeatherMiddleware) OnEvent(ctx context.Context, e *middleware.Event) (middleware.Decision, error) {
	if e.Name == middleware.EventBeforeLLMRequest {
		result := m.nluEngine.Parse(e.UserText)
		if result.Intent == "get_weather" {
			loc, _ := result.Slots["location"]
			return m.handleWeather(ctx, loc)
		}

		// Inject tool schema for LLM
		params := &middleware.LLMParams{}
		if e.Params != nil {
			*params = *e.Params
		}
		params.Tools = append(params.Tools, weatherTool())
		if params.ToolChoice == nil {
			params.ToolChoice = "auto"
		}
		return middleware.Decision{OverrideParams: params, Reason: "weather: injected tool schema"}, nil
	}

	if e.Name == middleware.EventAfterLLMResponse {
		calls, _ := e.Context["tool_calls"].([]middleware.ToolCall)
		for _, tc := range calls {
			if tc.Tool == "get_weather" {
				loc, _ := tc.Args["location"].(string)
				return m.handleWeather(ctx, loc)
			}
		}
	}

	return middleware.Decision{}, nil
}

func weatherTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "get_weather",
			Description: "Get the current weather for a specific location.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "The city and country, e.g., San Francisco, CA",
					},
				},
				"required": []string{"location"},
			},
		},
	}
}

func (m *WeatherMiddleware) handleWeather(ctx context.Context, loc string) (middleware.Decision, error) {
	if loc == "" {
		loc = "Berlin"
	}

	// 1. Geocoding to get Lat/Lon
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json", url.QueryEscape(loc))
	resp, err := http.Get(geoURL)
	if err != nil {
		return middleware.Decision{}, fmt.Errorf("geocoding error: %w", err)
	}
	defer resp.Body.Close()

	var geoResult struct {
		Results []struct {
			Lat float64 `json:"latitude"`
			Lon float64 `json:"longitude"`
			Nm  string  `json:"name"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geoResult); err != nil {
		return middleware.Decision{}, fmt.Errorf("geocoding decode error: %w", err)
	}

	if len(geoResult.Results) == 0 {
		msg := fmt.Sprintf("Could not find location: %s", loc)
		return middleware.Decision{Cancel: true, ReplaceText: &msg, Reason: "weather: location not found"}, nil
	}

	lat := geoResult.Results[0].Lat
	lon := geoResult.Results[0].Lon
	actualName := geoResult.Results[0].Nm

	// 2. Fetch Weather
	weatherURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true", lat, lon)
	wResp, err := http.Get(weatherURL)
	if err != nil {
		return middleware.Decision{}, fmt.Errorf("weather api error: %w", err)
	}
	defer wResp.Body.Close()

	var weatherResult struct {
		Current struct {
			Temp      float64 `json:"temperature"`
			WindSpeed float64 `json:"windspeed"`
			Code      int     `json:"weathercode"`
		} `json:"current_weather"`
	}
	if err := json.NewDecoder(wResp.Body).Decode(&weatherResult); err != nil {
		return middleware.Decision{}, fmt.Errorf("weather decode error: %w", err)
	}

	// Map weather code to description (simplified)
	condition := "Unknown"
	switch weatherResult.Current.Code {
	case 0:
		condition = "Clear sky"
	case 1, 2, 3:
		condition = "Partly cloudy"
	case 45, 48:
		condition = "Fog"
	case 51, 53, 55:
		condition = "Drizzle"
	case 61, 63, 65:
		condition = "Rain"
	case 71, 73, 75:
		condition = "Snow"
	case 95, 96, 99:
		condition = "Thunderstorm"
	}

	respData := map[string]any{
		"location":    actualName,
		"condition":   condition,
		"temperature": weatherResult.Current.Temp,
		"unit":        "°C",
		"wind_speed":  weatherResult.Current.WindSpeed,
		"message":     fmt.Sprintf("The current weather in %s is %s with %.1f°C and wind speed of %.1f km/h.", actualName, condition, weatherResult.Current.Temp, weatherResult.Current.WindSpeed),
	}

	jsonBytes, err := json.Marshal(respData)
	if err != nil {
		return middleware.Decision{}, err
	}

	respStr := string(jsonBytes)
	return middleware.Decision{
		Cancel:      true,
		ReplaceText: &respStr,
		Reason:      "weather: fetched live data from Open-Meteo",
	}, nil
}
