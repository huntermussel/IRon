package nlurouter

import (
	"iron/middlewares/nlurouter/handlers"
	"testing"
)

func TestNLURouter_ProcessQuery(t *testing.T) {
	r := NewRouter()
	r.Register("GetWeather", &handlers.WeatherHandler{})
	r.Register("SetAlarm", &handlers.AlarmHandler{})
	r.Register("DeviceControl", &handlers.DeviceHandler{})

	tests := []struct {
		input       string
		wantHandled bool
		wantComplex bool
		wantIntent  string
	}{
		{"What is the weather in Paris?", true, false, "GetWeather"},
		{"Set alarm for 7am", true, false, "SetAlarm"},
		{"Turn on the lights", true, false, "DeviceControl"},
		{"Tell me a joke", false, false, ""},
		{"How does the weather look like tomorrow?", false, true, "GetWeather"}, // Matches "weather" keyword but regex fails -> Complex
	}

	for _, tt := range tests {
		resp, handled, complexIntent, saved, prompt := r.ProcessQuery(tt.input)

		if handled != tt.wantHandled {
			t.Errorf("ProcessQuery(%q) handled = %v, want %v", tt.input, handled, tt.wantHandled)
		}

		if complexIntent != tt.wantComplex {
			t.Errorf("ProcessQuery(%q) complex = %v, want %v", tt.input, complexIntent, tt.wantComplex)
		}

		if handled {
			if saved <= 0 {
				t.Errorf("ProcessQuery(%q) saved tokens = %d, want > 0", tt.input, saved)
			}
			if resp == "" {
				t.Errorf("ProcessQuery(%q) returned empty response", tt.input)
			}
		}

		if complexIntent {
			if prompt == "" {
				t.Errorf("ProcessQuery(%q) returned empty system prompt for complex intent", tt.input)
			}
		}
	}
}
