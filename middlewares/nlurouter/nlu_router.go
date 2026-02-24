package nlurouter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	mw "iron/internal/middleware"
	"iron/middlewares/nlurouter/handlers"
	"iron/middlewares/nlurouter/nlp"
)

func init() {
	r := NewRouter()
	// Register default intents
	r.Register("GetWeather", &handlers.WeatherHandler{})
	r.Register("SetAlarm", &handlers.AlarmHandler{})
	r.Register("DeviceControl", &handlers.DeviceHandler{})

	mw.Register(r)
}

// IntentHandler defines the interface for handling specific intents.
type IntentHandler interface {
	// Match checks if the input matches the intent and returns extracted slots and confidence.
	Match(input string, tokens []nlp.Token) (bool, map[string]string, float64)
	// Handle executes the local logic for the intent.
	Handle(slots map[string]string) (any, error)
	// SystemPrompt returns a specific system prompt if this intent is detected but handled by LLM (hybrid/fallback).
	SystemPrompt() string
}

// NLURouter intercepts incoming natural language strings to classify intent and extract entities.
type NLURouter struct {
	mu       sync.RWMutex
	registry map[string]IntentHandler
	tagger   *nlp.SimplePOSTagger
}

// NewRouter creates a new NLU router with an empty registry and initializes the POS tagger.
func NewRouter() *NLURouter {
	return &NLURouter{
		registry: make(map[string]IntentHandler),
		tagger:   nlp.NewPOSTagger(),
	}
}

// Register adds an intent handler to the router.
func (n *NLURouter) Register(name string, handler IntentHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.registry[name] = handler
}

// Middleware interface implementation

func (n *NLURouter) ID() string    { return "nlu_router" }
func (n *NLURouter) Priority() int { return 120 } // High priority to intercept early

func (n *NLURouter) ShouldLoad(_ context.Context, e *mw.Event) bool {
	// Enable by default, or check context flag
	if e != nil && e.Context != nil {
		if v, ok := e.Context["nlu_router"].(bool); ok {
			return v
		}
	}
	return true
}

func (n *NLURouter) OnEvent(ctx context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	input := strings.TrimSpace(e.UserText)
	if input == "" {
		return mw.Decision{}, nil
	}

	resp, handled, complexIntent, savedTokens, sysPrompt := n.ProcessQuery(input)

	if handled {
		// Return JSON response (bypass LLM)
		return mw.Decision{
			Cancel:      true,
			ReplaceText: &resp,
			Reason:      fmt.Sprintf("nlu_router: handled locally (saved ~%d tokens)", savedTokens),
		}, nil
	}

	if complexIntent {
		// Hybrid Routing: Forward to LLM with specialized System Prompt
		// We prepend the system prompt to the user text as a hint, since Service ignores Params.Messages
		newText := fmt.Sprintf("[System: %s]\n%s", sysPrompt, input)

		// Disable intent_compressor to prevent it from mangling the injected system prompt
		if e.Context == nil {
			e.Context = make(map[string]any)
		}
		e.Context["intent_compressor"] = false

		return mw.Decision{
			ReplaceText: &newText,
			Reason:      "nlu_router: injected context for complex intent",
		}, nil
	}

	return mw.Decision{}, nil
}

// ProcessQuery normalizes text, identifies intent, extracts slots, and executes handler if applicable.
// Returns:
// - response: JSON string if handled locally
// - handled: true if handled locally
// - complex: true if intent matched but requires LLM
// - savedTokens: estimated tokens saved
// - systemPrompt: prompt to inject if complex
func (n *NLURouter) ProcessQuery(input string) (string, bool, bool, int, string) {
	normInput := strings.ToLower(input)

	// POS Tagging
	tokens := n.tagger.Tag(normInput)

	n.mu.RLock()
	defer n.mu.RUnlock()

	var bestIntent string
	var bestHandler IntentHandler
	var bestSlots map[string]string
	var maxConf float64

	for name, handler := range n.registry {
		matched, slots, conf := handler.Match(normInput, tokens)
		if matched && conf > maxConf {
			maxConf = conf
			bestIntent = name
			bestHandler = handler
			bestSlots = slots
		}
	}

	const executionThreshold = 0.9
	const contextThreshold = 0.5

	// Calculate estimated tokens (char count / 4)
	savedTokens := len(input) / 4
	if savedTokens < 1 {
		savedTokens = 1
	}

	if maxConf >= executionThreshold {
		respData, err := bestHandler.Handle(bestSlots)
		if err != nil {
			// Fallback to LLM if execution fails
			return "", false, true, 0, bestHandler.SystemPrompt()
		}

		// Structure response
		result := map[string]any{
			"intent":       bestIntent,
			"data":         respData,
			"source":       "local_nlu",
			"saved_tokens": savedTokens,
			"slots":        bestSlots,
		}
		jsonBytes, _ := json.Marshal(result)
		return string(jsonBytes), true, false, savedTokens, ""
	}

	if maxConf >= contextThreshold {
		return "", false, true, 0, bestHandler.SystemPrompt()
	}

	return "", false, false, 0, ""
}
