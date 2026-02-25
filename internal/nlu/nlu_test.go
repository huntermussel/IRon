package nlu

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	e := &Engine{
		matchers: make([]*intentMatcher, 0),
	}

	e.RegisterIntent("set_alarm",
		"set alarm for {time}",
		"wake me up at {time}",
		"remind me to {action} at {time}",
	)

	e.RegisterIntent("set_cron",
		"remind me to {action} every {duration}",
		"run {action} every {duration}",
		"{action} every {duration}",
		"{action} {count} times per {unit}",
		"{action} {count} time per {unit}",
	)

	e.RegisterIntent("get_weather",
		"weather in {location}",
		"what is the weather in {location}",
	)

	e.RegisterIntent("control_device",
		"turn {state} the {device}",
		"turn {state} {device}",
	)

	tests := []struct {
		input          string
		expectedIntent string
		expectedSlots  map[string]string
	}{
		{
			input:          "set alarm for 8am",
			expectedIntent: "set_alarm",
			expectedSlots:  map[string]string{"time": "8am"},
		},
		{
			input:          "remind me to take water every 2 hours",
			expectedIntent: "set_cron",
			expectedSlots:  map[string]string{"action": "take water", "duration": "2 hours"},
		},
		{
			input:          "search every hour",
			expectedIntent: "set_cron",
			expectedSlots:  map[string]string{"action": "search", "duration": "hour"},
		},
		{
			input:          "search 1 time per week",
			expectedIntent: "set_cron",
			expectedSlots:  map[string]string{"action": "search", "count": "1", "unit": "week"},
		},
		{
			input:          "weather in Paris",
			expectedIntent: "get_weather",
			expectedSlots:  map[string]string{"location": "Paris"},
		},
		{
			input:          "turn on the lights",
			expectedIntent: "control_device",
			expectedSlots:  map[string]string{"state": "on", "device": "lights"},
		},
		{
			input:          "what is the weather",
			expectedIntent: "",
			expectedSlots:  nil,
		},
	}

	for _, tt := range tests {
		result := e.Parse(tt.input)
		if result.Intent != tt.expectedIntent {
			t.Errorf("Parse(%q): expected intent %q, got %q", tt.input, tt.expectedIntent, result.Intent)
		}
		if result.Slots != nil && !reflect.DeepEqual(result.Slots, tt.expectedSlots) {
			t.Errorf("Parse(%q): expected slots %v, got %v", tt.input, tt.expectedSlots, result.Slots)
		}
	}
}
