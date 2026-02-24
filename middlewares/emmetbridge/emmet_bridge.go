package emmetbridge

import (
	"context"
	"strings"

	mw "iron/internal/middleware"
)

func init() {
	mw.Register(EmmetBridge{})
}

// EmmetBridge converts HTML→Emmet on the way in (asks LLM to speak Emmet),
// and Emmet→HTML on the way out.
type EmmetBridge struct{}

func (EmmetBridge) ID() string    { return "emmet_bridge" }
func (EmmetBridge) Priority() int { return 95 }

func (EmmetBridge) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["emmet_bridge"].(bool); ok {
		return v
	}
	return true
}

func (EmmetBridge) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil {
		return mw.Decision{}, nil
	}

	switch e.Name {
	case mw.EventBeforeLLMRequest:
		html := strings.TrimSpace(e.UserText)
		if !looksLikeHTML(html) {
			return mw.Decision{}, nil
		}
		emmet := htmlToEmmet(html)
		if emmet == "" {
			return mw.Decision{}, nil
		}
		prompt := "if HTML respond in Emmet only" + emmet
		return mw.Decision{
			ReplaceText: &prompt,
			Reason:      "emmet_bridge: html→emmet",
		}, nil

	case mw.EventAfterLLMResponse:
		out := strings.TrimSpace(e.LLMText)
		if !looksLikeEmmet(out) {
			return mw.Decision{}, nil
		}
		html := emmetToHTML(out)
		if html == "" {
			return mw.Decision{}, nil
		}
		return mw.Decision{
			ReplaceText: &html,
			Reason:      "emmet_bridge: emmet→html",
		}, nil
	}
	return mw.Decision{}, nil
}
