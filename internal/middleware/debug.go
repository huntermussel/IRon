package middleware

import (
	"encoding/json"
	"io"
	"math"
	"regexp"
	"time"
	"unicode/utf8"
)

type debugEntry struct {
	Timestamp    string `json:"ts"`
	Event        string `json:"event"`
	MiddlewareID string `json:"middleware"`
	Priority     int    `json:"priority"`
	Skipped      bool   `json:"skipped,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Cancel       bool   `json:"cancel,omitempty"`

	InputChars   int `json:"in_chars"`
	OutputChars  int `json:"out_chars"`
	InputTokens  int `json:"in_tokens_est"`
	OutputTokens int `json:"out_tokens_est"`

	SavedTokens int     `json:"saved_tokens_est"`
	SavedPct    float64 `json:"saved_pct,omitempty"`
}

// tokenish matches "word-like" chunks (including dotted/slashed technical tokens),
// otherwise falls back to single non-space characters.
var tokenish = regexp.MustCompile(`[\pL\pN]+(?:[._/\\-][\pL\pN]+)*|[^\s]`)

func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	// A simple approximation: count token-ish chunks; cap the minimum by a
	// chars/4 heuristic so tiny punctuation-heavy strings don't look too cheap.
	chunks := len(tokenish.FindAllString(s, -1))
	charHeuristic := int(math.Ceil(float64(utf8.RuneCountInString(s)) / 4.0))
	if chunks < charHeuristic {
		return charHeuristic
	}
	return chunks
}

func eventText(e *Event) string {
	if e == nil {
		return ""
	}
	switch e.Name {
	case EventBeforeLLMRequest:
		return e.UserText
	case EventAfterLLMResponse, EventBeforeUserReply:
		return e.LLMText
	default:
		return ""
	}
}

func applyDecisionToEvent(e *Event, dec Decision) {
	if e == nil {
		return
	}
	if dec.OverrideParams != nil {
		e.Params = dec.OverrideParams
	}
	if dec.ReplaceText == nil {
		return
	}
	switch e.Name {
	case EventBeforeLLMRequest:
		e.UserText = *dec.ReplaceText
	case EventAfterLLMResponse, EventBeforeUserReply:
		e.LLMText = *dec.ReplaceText
	}
}

func (c *Chain) debugLog(e *Event, id string, priority int, skipped bool, inText, outText string, dec Decision) {
	c.debugMu.Lock()
	w := c.debugW
	c.debugMu.Unlock()
	if w == nil {
		return
	}

	inChars := utf8.RuneCountInString(inText)
	outChars := utf8.RuneCountInString(outText)
	inTok := estimateTokens(inText)
	outTok := estimateTokens(outText)

	saved := inTok - outTok
	var savedPct float64
	if inTok > 0 {
		savedPct = float64(saved) / float64(inTok)
	}

	entry := debugEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		Event:        string(e.Name),
		MiddlewareID: id,
		Priority:     priority,
		Skipped:      skipped,
		Reason:       dec.Reason,
		Cancel:       dec.Cancel,
		InputChars:   inChars,
		OutputChars:  outChars,
		InputTokens:  inTok,
		OutputTokens: outTok,
		SavedTokens:  saved,
		SavedPct:     savedPct,
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = io.WriteString(w, string(b)+"\n")
}
