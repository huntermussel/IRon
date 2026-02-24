package trashcleanner

import (
	"context"
	mw "iron/internal/middleware"
	"regexp"
	"strings"
	"unicode/utf8"
)

func init() {
	// Auto-register this middleware so the core can load it dynamically.
	mw.Register(TrashCleaner{})
}

// TrashCleaner aggressively compresses user text for LLM input by:
// - Protecting technical spans (code, URLs, emails, paths, flags, JSON/YAML-like blocks)
// - Stripping greetings / sign-offs / fluff phrases (EN+PT)
// - Removing low-signal stopwords (EN+PT) while preserving negations and constraints
// - Optionally applying shorthand and constraint canonicalization (KEEP/RM/NOCHANGE/FIX)
//
// Enable per request by setting Event.Context["trash_cleaner"]=true.
// Modes:
//   - "safe"  : only greetings/fluff + whitespace cleanup
//   - "aggr"  : + stopwords + constraint canonicalization
//   - "ultra" : + shorthands (most token reduction)
//
// Set Event.Context["trash_cleaner_mode"]="safe|aggr|ultra".
type TrashCleaner struct{}

func (TrashCleaner) ID() string    { return "trash_cleaner" }
func (TrashCleaner) Priority() int { return 100 }

// ShouldLoad returns true by default. If Event.Context["trash_cleaner"] is set
// to a boolean, it will respect that flag (false disables).
func (TrashCleaner) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["trash_cleaner"].(bool); ok {
		return v
	}
	return true
}

func (TrashCleaner) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	mode := "aggr"
	if e.Context != nil {
		if v, ok := e.Context["trash_cleaner_mode"].(string); ok && v != "" {
			mode = strings.ToLower(strings.TrimSpace(v))
		}
	}

	orig := strings.TrimSpace(e.UserText)
	if orig == "" {
		return mw.Decision{}, nil
	}

	clean := compressPragmatic(orig, mode)

	// Guardrail: if we nuked the prompt into emptiness, do not cancel the request.
	// Just fall back to original.
	if strings.TrimSpace(clean) == "" {
		return mw.Decision{}, nil
	}

	// If nothing changed, no-op.
	if clean == orig {
		return mw.Decision{}, nil
	}

	return mw.Decision{ReplaceText: &clean, Reason: "trash_cleaner: compressed user text"}, nil
}

/* ---------------------------- Span protection ---------------------------- */

// We protect things that must NOT be altered or re-spaced.
// Placeholders are ASCII-ish and unlikely to appear in normal text.
const phPrefix = "⟦P"
const phSuffix = "⟧"

type protected struct {
	ph    string
	value string
}

var (
	reFenceCode = regexp.MustCompile("(?s)```.*?```")
	reInline    = regexp.MustCompile("`[^`\n]+`")

	// URLs (broad but practical)
	reURL = regexp.MustCompile(`\bhttps?://[^\s<>()\[\]{}"'` + "`" + `]+`)

	// Email
	reEmail = regexp.MustCompile(`\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

	// UNIX paths and Windows paths (approx)
	reUnixPath = regexp.MustCompile(`(?m)(^|\s)(/[^ \t\r\n]+)`)
	reWinPath  = regexp.MustCompile(`(?m)\b[A-Za-z]:\\[^ \t\r\n]+`)

	// CLI flags like --foo=bar, -n, --no-cache
	reFlag = regexp.MustCompile(`\B--?[A-Za-z][A-Za-z0-9\-]*(?:=[^\s]+)?`)

	// JSON-ish / YAML-ish blocks (best-effort)
	reJSONLike = regexp.MustCompile(`(?s)\{.*?\}`)
	reYAMLLike = regexp.MustCompile(`(?m)^[ \t]*[A-Za-z0-9_\-]+:[ \t]*.*$`)

	// Case-insensitive wrapper (Go RE2: use inline (?i))
	_ = reEmail
)

func protectSpans(s string) (string, []protected) {
	var items []protected
	i := 0

	apply := func(re *regexp.Regexp, input string) string {
		return re.ReplaceAllStringFunc(input, func(m string) string {
			ph := phPrefix + itoa(i) + phSuffix
			items = append(items, protected{ph: ph, value: m})
			i++
			return ph
		})
	}

	// Order matters: bigger constructs first.
	s = apply(reFenceCode, s)
	s = apply(reInline, s)
	s = apply(reURL, s)

	// Emails should be case-insensitive; do it by uppercasing a shadow match approach:
	// Since RE2 doesn't support inline flags per-subpattern easily in this context,
	// we do a replace pass by scanning tokens. Simpler: create a case-insensitive regex.
	reEmailCI := regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	s = apply(reEmailCI, s)

	s = apply(reWinPath, s)
	s = apply(reUnixPath, s)
	s = apply(reFlag, s)

	// Protect JSON/YAML only in aggressive modes? Keeping always is safer.
	s = apply(reJSONLike, s)
	s = apply(reYAMLLike, s)

	return s, items
}

func restoreSpans(s string, items []protected) string {
	// Replace placeholders back (reverse order is safer if any nesting happened).
	for j := len(items) - 1; j >= 0; j-- {
		s = strings.ReplaceAll(s, items[j].ph, items[j].value)
	}
	return s
}

/* ---------------------------- Text compression --------------------------- */

var (
	// Remove control chars except \n and \t
	reCtl = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]+`)

	// Collapse whitespace
	reWS = regexp.MustCompile(`[ \t]+`)
	reNL = regexp.MustCompile(`\n{3,}`)

	// Ornamental punctuation runs
	rePunctRuns = regexp.MustCompile(`[!?.]{3,}`)

	// Greetings (start) EN+PT
	reGreeting = regexp.MustCompile(`(?i)^\s*(hi|hello|hey|yo|oi|olá|ola|bom dia|boa tarde|boa noite)\b[,\s!.:;-]*`)

	// Sign-offs (end) EN+PT
	reSignOff = regexp.MustCompile(`(?i)[,\s\-]*(thanks|tks|thx|thank you|valeu|obrigado|obg|abraços|abs)\b[.!?\s]*$`)

	// Fluff phrases (remove anywhere, but conservative patterns)
	reFluff = regexp.MustCompile(`(?i)\b(please|por favor|if you can|se puder|could you|can you|would you|i want you to|i would like you to|i need you to|preciso que você|quero que você|gostaria que você)\b`)

	// Word boundaries for stopword stripping (placeholder tokens are protected already).
	reWord = regexp.MustCompile(`\b[\p{L}\p{N}']+\b`)

	// Constraint patterns -> canonical tokens
	reNoChangeEN = regexp.MustCompile(`(?i)\b(do not|don't|dont|never)\s+(change|alter|modify)\b`)
	reNoChangePT = regexp.MustCompile(`(?i)\b(não|nao)\s+(alter(e|ar)|mude|modifique)\b`)
	reKeepEN     = regexp.MustCompile(`(?i)\b(keep|preserve|maintain)\b`)
	reKeepPT     = regexp.MustCompile(`(?i)\b(mantenha|preserve)\b`)
	reRemoveEN   = regexp.MustCompile(`(?i)\b(remove|delete|strip)\b`)
	reRemovePT   = regexp.MustCompile(`(?i)\b(remova|remover|apague)\b`)
	reFixEN      = regexp.MustCompile(`(?i)\b(fix|adjust|correct|improve)\b`)
	reFixPT      = regexp.MustCompile(`(?i)\b(corrija|ajuste|melhore)\b`)
)

// Stopwords (EN+PT). Keep lists should override removals.
var stopEN = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "so": {},
	"to": {}, "of": {}, "in": {}, "on": {}, "at": {}, "by": {}, "for": {}, "from": {}, "with": {}, "into": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"it": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"i": {}, "you": {}, "we": {}, "they": {}, "he": {}, "she": {}, "me": {}, "my": {}, "your": {}, "our": {}, "their": {},
	"as": {}, "if": {}, "then": {}, "than": {}, "because": {},
	"just": {}, "really": {}, "very": {}, "maybe": {}, "basically": {},
}

var stopPT = map[string]struct{}{
	"o": {}, "a": {}, "os": {}, "as": {}, "um": {}, "uma": {}, "uns": {}, "umas": {},
	"e": {}, "ou": {}, "mas": {}, "então": {}, "entao": {},
	"de": {}, "da": {}, "do": {}, "das": {}, "dos": {}, "em": {}, "no": {}, "na": {}, "nos": {}, "nas": {},
	"para": {}, "por": {}, "com": {}, "sobre": {}, "entre": {}, "até": {}, "ate": {},
	"é": {}, "eh": {}, "são": {}, "sao": {}, "foi": {}, "eram": {}, "ser": {}, "sendo": {},
	"isso": {}, "isto": {}, "aquilo": {}, "esse": {}, "essa": {}, "esses": {}, "essas": {},
	"eu": {}, "você": {}, "voce": {}, "nós": {}, "eles": {}, "elas": {}, "meu": {}, "minha": {}, "seu": {}, "sua": {},
	"como": {}, "se": {}, "porque": {}, "pra": {},
	"tipo": {}, "meio": {}, "basicamente": {}, "realmente": {}, "muito": {},
}

// Never drop these (constraints, negations, quantifiers).
var keep = map[string]struct{}{
	// EN
	"not": {}, "no": {}, "never": {}, "without": {}, "except": {}, "only": {},
	"must": {}, "mustn't": {}, "cant": {}, "can't": {}, "cannot": {}, "avoid": {}, "exclude": {}, "omit": {},
	// PT
	"não": {}, "nao": {}, "nunca": {}, "sem": {}, "exceto": {}, "somente": {}, "apenas": {}, "só": {}, "so": {},
	"obrigatório": {}, "obrigatorio": {}, "evite": {}, "exclua": {}, "omita": {},
}

// Ultra shorthand mapping (apply only on free text).
var ultraSh = map[string]string{
	"example": "ex", "examples": "exs", "exemplo": "ex", "exemplos": "exs",
	"configuration": "cfg", "configuração": "cfg", "configuracao": "cfg",
	"documentation": "docs", "documentação": "docs", "documentacao": "docs",
	"parameters": "params", "parâmetros": "params", "parametros": "params",
	"message": "msg", "mensagem": "msg",
	"response": "resp", "resposta": "resp",
	"requirements": "reqs", "requisitos": "reqs",
	"constraint": "cstr", "restrição": "cstr", "restricao": "cstr",
	"performance": "perf", "latency": "lat", "memory": "mem",
}

func compressPragmatic(input, mode string) string {
	// 1) Protect technical spans.
	s, prot := protectSpans(input)

	// 2) Normalize noise.
	s = reCtl.ReplaceAllString(s, "")
	s = rePunctRuns.ReplaceAllString(s, ".")
	s = reGreeting.ReplaceAllString(s, "")
	s = reSignOff.ReplaceAllString(s, "")
	s = reFluff.ReplaceAllString(s, " ")

	// 3) Canonicalize constraint verbs (aggr+).
	if mode == "aggr" || mode == "ultra" {
		s = canonicalizeConstraints(s)
	}

	// 4) Stopword stripping (aggr+), but never touch placeholders.
	if mode == "aggr" || mode == "ultra" {
		s = stripStopwords(s)
	}

	// 5) Ultra shorthand (ultra only).
	if mode == "ultra" {
		s = applyShorthand(s)
	}

	// 6) Final whitespace cleanup.
	s = reWS.ReplaceAllString(strings.TrimSpace(s), " ")
	s = reNL.ReplaceAllString(s, "\n\n")

	// 7) Restore technical spans.
	s = restoreSpans(s, prot)

	// Final trim.
	return strings.TrimSpace(s)
}

func canonicalizeConstraints(s string) string {
	// NOCHANGE
	s = reNoChangeEN.ReplaceAllString(s, "NOCHANGE")
	s = reNoChangePT.ReplaceAllString(s, "NOCHANGE")

	// KEEP/RM/FIX verbs -> tokens (keeps text after them intact)
	s = reKeepEN.ReplaceAllString(s, "KEEP")
	s = reKeepPT.ReplaceAllString(s, "KEEP")
	s = reRemoveEN.ReplaceAllString(s, "RM")
	s = reRemovePT.ReplaceAllString(s, "RM")
	s = reFixEN.ReplaceAllString(s, "FIX")
	s = reFixPT.ReplaceAllString(s, "FIX")
	return s
}

func stripStopwords(s string) string {
	// Avoid touching placeholders; they are not word chars anyway, but be safe:
	// Split by spaces and process word tokens with minimal transformations.
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		// Keep placeholders verbatim.
		if strings.HasPrefix(p, phPrefix) && strings.HasSuffix(p, phSuffix) {
			out = append(out, p)
			continue
		}

		low := strings.ToLower(p)

		// Keep constraint tokens and negations.
		if _, ok := keep[low]; ok {
			out = append(out, low)
			continue
		}
		if low == "nochange" || low == "keep" || low == "rm" || low == "fix" {
			out = append(out, strings.ToUpper(low))
			continue
		}

		// Drop stopwords (EN+PT). If unknown token, keep.
		if _, ok := stopEN[low]; ok {
			continue
		}
		if _, ok := stopPT[low]; ok {
			continue
		}

		out = append(out, low)
	}

	return strings.Join(out, " ")
}

func applyShorthand(s string) string {
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if strings.HasPrefix(p, phPrefix) && strings.HasSuffix(p, phSuffix) {
			out = append(out, p)
			continue
		}
		// Preserve canonical tokens.
		if p == "NOCHANGE" || p == "KEEP" || p == "RM" || p == "FIX" {
			out = append(out, p)
			continue
		}

		low := strings.ToLower(p)
		if rep, ok := ultraSh[low]; ok {
			out = append(out, rep)
		} else {
			out = append(out, low)
		}
	}

	return strings.Join(out, " ")
}

/* ------------------------------ Small utils ------------------------------ */

// itoa without strconv (tiny + fast). Assumes i >= 0.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [32]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + (i % 10))
		i /= 10
	}
	return string(b[pos:])
}

// Safe rune count (not currently used, but handy if you add guardrails).
func runeLen(s string) int {
	if s == "" {
		return 0
	}
	return utf8.RuneCountInString(s)
}
