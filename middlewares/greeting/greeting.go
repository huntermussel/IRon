package greeting

import (
	"context"
	"strings"
	"unicode"

	mw "iron/internal/middleware"
)

func init() {
	mw.Register(Greeting{})
}

// Greeting intercepts simple salutations and responds immediately without
// hitting the LLM.
type Greeting struct{}

func (Greeting) ID() string    { return "greeting" }
func (Greeting) Priority() int { return 110 } // run early

func (Greeting) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["greeting"].(bool); ok {
		return v
	}
	return true
}

func (Greeting) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}
	user := strings.TrimSpace(e.UserText)
	if user == "" {
		return mw.Decision{}, nil
	}

	if isGreetingOnly(user) {
		reply := "Hi, how can I assist you today?"
		return mw.Decision{
			Cancel:      true,
			ReplaceText: &reply,
			Reason:      "greeting",
		}, nil
	}
	return mw.Decision{}, nil
}

/* ---------------------------- Helpers ---------------------------- */

var greetWords = map[string]struct{}{
	"hi": {}, "hello": {}, "hey": {}, "heya": {}, "howdy": {}, "yo": {},
	"good": {}, "morning": {}, "afternoon": {}, "evening": {}, "greetings": {},
}

func isGreetingOnly(s string) bool {
	// strip leading/trailing punctuation
	s = strings.TrimSpace(stripPunct(s))
	if s == "" {
		return false
	}
	words := strings.Fields(s)
	if len(words) == 0 || len(words) > 4 {
		return false
	}

	for i, w := range words {
		w = strings.ToLower(stripPunct(w))
		if w == "" {
			return false
		}
		// allow "good morning"/"good evening"
		if _, ok := greetWords[w]; ok {
			continue
		}
		// allow polite filler
		if (w == "there" && i == len(words)-1) || w == "hi!" {
			continue
		}
		return false
	}
	return true
}

func stripPunct(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPunct(r) && r != '\'' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
