package alarm

import (
	"context"
	"encoding/json"
	"fmt"

	"iron/internal/middleware"
	"iron/internal/nlu"
)

// AlarmMiddleware handles alarm setting requests.
type AlarmMiddleware struct {
	nluEngine *nlu.Engine
}

func init() {
	middleware.Register(New())
}

func New() *AlarmMiddleware {
	engine := nlu.GetEngine()
	// Register structured utterances for this intent
	engine.RegisterIntent("set_alarm",
		"set alarm for {time}",
		"set an alarm for {time}",
		"wake me up at {time}",
		"wake me up {time}",
		"create alarm for {time}",
		"alarm {time}",
		"at {time} I'll {action}",
	)

	return &AlarmMiddleware{
		nluEngine: engine,
	}
}

func (m *AlarmMiddleware) ID() string {
	return "alarm"
}

func (m *AlarmMiddleware) Priority() int {
	return 100
}

// ShouldLoad checks if the input matches the alarm intent using the shared NLU engine.
func (m *AlarmMiddleware) ShouldLoad(ctx context.Context, e *middleware.Event) bool {
	if e.Name != middleware.EventBeforeLLMRequest {
		return false
	}

	result := m.nluEngine.Parse(e.UserText)
	return result.Intent == "set_alarm"
}

func (m *AlarmMiddleware) OnEvent(ctx context.Context, e *middleware.Event) (middleware.Decision, error) {
	// Parse again to get the result (or we could cache it in Context if we wanted optimization)
	result := m.nluEngine.Parse(e.UserText)

	timeStr := "unknown time"
	if t, ok := result.Slots["time"]; ok {
		timeStr = t
	}

	respData := map[string]any{
		"action":  "set_alarm",
		"time":    timeStr,
		"status":  "success",
		"message": fmt.Sprintf("Alarm set for %s.", timeStr),
	}

	jsonBytes, err := json.Marshal(respData)
	if err != nil {
		return middleware.Decision{}, err
	}

	respStr := string(jsonBytes)
	return middleware.Decision{
		Cancel:      true,
		ReplaceText: &respStr,
		Reason:      "alarm: handled locally via NLU match",
	}, nil
}
