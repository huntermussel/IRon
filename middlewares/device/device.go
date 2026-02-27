package device

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"iron/internal/middleware"
	"iron/internal/nlu"
)

// DeviceMiddleware handles device control requests.
type DeviceMiddleware struct {
	nluEngine *nlu.Engine
}

func init() {
	middleware.Register(New())
}

func New() *DeviceMiddleware {
	engine := nlu.GetEngine()
	// Register structured utterances for this intent
	// The order of utterances matters for priority if regexes overlap.
	// Specific patterns first.
	engine.RegisterIntent("control_device",
		"turn {state} the {device}",
		"turn {state} {device}",
		"switch {state} the {device}",
		"switch {state} {device}",
		"turn the {device} {state}",
		"turn {device} {state}",
	)

	return &DeviceMiddleware{
		nluEngine: engine,
	}
}

func (m *DeviceMiddleware) ID() string {
	return "device"
}

func (m *DeviceMiddleware) Priority() int {
	return 110
}

// ShouldLoad checks if the input matches the device intent using the shared NLU engine.
func (m *DeviceMiddleware) ShouldLoad(ctx context.Context, e *middleware.Event) bool {
	if e.Name != middleware.EventBeforeLLMRequest {
		return false
	}

	result := m.nluEngine.Parse(e.UserText)
	return result.Intent == "control_device"
}

func (m *DeviceMiddleware) OnEvent(ctx context.Context, e *middleware.Event) (middleware.Decision, error) {
	result := m.nluEngine.Parse(e.UserText)

	device := "unknown device"
	state := "unknown state"

	if d, ok := result.Slots["device"]; ok {
		device = d
	}
	if s, ok := result.Slots["state"]; ok {
		state = s
	}

	// Virtual House Persistence: updates a local JSON file to track device states
	home, _ := os.UserHomeDir()
	housePath := filepath.Join(home, ".iron", "virtual_house.json")
	os.MkdirAll(filepath.Dir(housePath), 0755)

	houseState := make(map[string]string)
	data, err := os.ReadFile(housePath)
	if err == nil {
		json.Unmarshal(data, &houseState)
	}

	houseState[device] = state
	newData, _ := json.MarshalIndent(houseState, "", "  ")
	os.WriteFile(housePath, newData, 0644)

	respData := map[string]any{
		"action":  "control_device",
		"device":  device,
		"state":   state,
		"status":  "success",
		"message": fmt.Sprintf("Turned %s the %s (state saved in Virtual House).", state, device),
		"house":   houseState,
	}

	jsonBytes, err := json.Marshal(respData)
	if err != nil {
		return middleware.Decision{}, err
	}

	respStr := string(jsonBytes)
	return middleware.Decision{
		Cancel:      true,
		ReplaceText: &respStr,
		Reason:      "device: handled locally via NLU match",
	}, nil
}
