package trashcleanner

import (
	"context"
	mw "iron/internal/middleware"
	"regexp"
	"strings"
	"unicode"
)

func init() {
	// Auto-register this middleware so the core can load it dynamically.
	mw.Register(TrashCleaner{})
}

// TrashCleaner is a middleware that compresses user requests by removing
// low-signal tokens (stopwords) while keeping negations and technical tokens.
//
// Enable per request by setting Event.Context["trash_cleaner"]=true.
type TrashCleaner struct{}

func (TrashCleaner) ID() string    { return "trash_cleaner" }
func (TrashCleaner) Priority() int { return 100 }

func (TrashCleaner) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	enabled, ok := e.Context["trash_cleaner"].(bool)
	return ok && enabled
}

func (TrashCleaner) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	clean := compressEnglishPrompt(e.UserText)
	if strings.TrimSpace(clean) == "" {
		return mw.Decision{Cancel: true, Reason: "empty after trash cleaning"}, nil
	}
	if clean == strings.TrimSpace(e.UserText) {
		return mw.Decision{}, nil
	}
	return mw.Decision{ReplaceText: &clean, Reason: "trash_cleaner: compressed user text"}, nil
}

// Minimal English stopword set (expand as needed).
var stopwordsEN = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "so": {},
	"to": {}, "of": {}, "in": {}, "on": {}, "at": {}, "by": {}, "for": {}, "from": {}, "with": {}, "into": {}, "over": {}, "under": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"it": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"i": {}, "you": {}, "we": {}, "they": {}, "he": {}, "she": {},
	"me": {}, "my": {}, "your": {}, "our": {}, "their": {},
	"as": {}, "if": {}, "then": {}, "than": {}, "because": {},
	"just": {}, "really": {}, "very": {}, "maybe": {}, "basically": {},
	"do": {}, "does": {}, "did": {},
}

// Words we should never drop (high semantic risk).
var keepEN = map[string]struct{}{
	"not": {}, "no": {}, "never": {}, "without": {}, "except": {}, "only": {},
}

// Heuristic: tokens that look technical should be preserved.
var techHint = regexp.MustCompile(`(^[vV]?\d+(\.\d+)+$)|([/_\-])|(^[a-z]+[0-9]+[a-z0-9-]*$)|(^[A-Z]{2,}$)`)

func isTechToken(tok string) bool {
	return techHint.MatchString(tok)
}

// Tokenize keeps alphanumerics and separates punctuation.
// Example: "Cloudflare Pages." -> ["Cloudflare", "Pages", "."]
func tokenize(s string) []string {
	var tokens []string
	var cur []rune

	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, string(cur))
			cur = cur[:0]
		}
	}

	rs := []rune(s)
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur = append(cur, r)
		case r == '.' && len(cur) > 0 && i+1 < len(rs) && (unicode.IsLetter(rs[i+1]) || unicode.IsDigit(rs[i+1])) && (unicode.IsLetter(cur[len(cur)-1]) || unicode.IsDigit(cur[len(cur)-1])):
			// Keep dots inside technical tokens (e.g., versions/domains): v2.1.0, example.com
			cur = append(cur, r)
		case unicode.IsSpace(r):
			flush()
		default:
			flush()
			tokens = append(tokens, string(r))
		}
	}
	flush()
	return tokens
}

var (
	spaceBeforePunct = regexp.MustCompile(`\s+([.,;:!?()\]\}])`)
	spaceAfterOpen   = regexp.MustCompile(`([(\[\{])\s+`)
	multiSpace       = regexp.MustCompile(`\s+`)
)

func compressEnglishPrompt(input string) string {
	toks := tokenize(input)

	out := make([]string, 0, len(toks))
	for _, t := range toks {
		// punctuation tokens: keep minimal set
		if len(t) == 1 && strings.ContainsRune(".,;:!?()[]{}", rune(t[0])) {
			out = append(out, t)
			continue
		}

		low := strings.ToLower(t)

		// keep critical words
		if _, ok := keepEN[low]; ok {
			out = append(out, low)
			continue
		}

		// drop stopwords unless token looks technical
		if _, isStop := stopwordsEN[low]; isStop && !isTechToken(t) {
			continue
		}

		// normalize non-technical tokens to lowercase
		if !isTechToken(t) {
			out = append(out, low)
		} else {
			out = append(out, t) // preserve casing for acronyms/ids
		}
	}

	result := strings.Join(out, " ")
	result = spaceBeforePunct.ReplaceAllString(result, "$1")
	result = spaceAfterOpen.ReplaceAllString(result, "$1")
	result = multiSpace.ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}
