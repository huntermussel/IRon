package intentcompressor

import (
	"context"
	mw "iron/internal/middleware"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

func init() {
	mw.Register(IntentCompressor{})
}

// IntentCompressor converts verbose user requests into short "intent
// abbreviations". Example: "build a landing page in react for an institutional
// site" -> "react landing: institutional".
//
// Enable per request via Event.Context["intent_compressor"]=true (default true).
// Optional:
//
//	Event.Context["intent_compressor_mode"]="safe|aggr" (default aggr)
//	Event.Context["intent_compressor_min_score"]=int (default 3)
//
// Notes:
// - Protects code/URLs from influencing intent detection.
// - Adds "cstr" suffix if constraints/negations are present.
type IntentCompressor struct{}

func (IntentCompressor) ID() string    { return "intent_compressor" }
func (IntentCompressor) Priority() int { return 90 }

func (IntentCompressor) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return true
	}
	if v, ok := e.Context["intent_compressor"].(bool); ok {
		return v
	}
	return true
}

func (IntentCompressor) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	orig := strings.TrimSpace(e.UserText)
	if orig == "" {
		return mw.Decision{}, nil
	}

	mode := "aggr"
	minScore := 3
	if e.Context != nil {
		if v, ok := e.Context["intent_compressor_mode"].(string); ok && v != "" {
			mode = strings.ToLower(strings.TrimSpace(v))
		}
		if v, ok := e.Context["intent_compressor_min_score"].(int); ok && v > 0 {
			minScore = v
		}
	}

	out := compressIntent(orig, mode, minScore)
	if out == "" || out == orig {
		return mw.Decision{}, nil
	}
	return mw.Decision{ReplaceText: &out, Reason: "intent_compressor: intent abbreviation"}, nil
}

/* ------------------------------ Core logic ------------------------------ */

type protected struct {
	ph    string
	value string
}

const phPrefix = "⟦I"
const phSuffix = "⟧"

var (
	reFenceCode = regexp.MustCompile("(?s)```.*?```")
	reInline    = regexp.MustCompile("`[^`\n]+`")
	reURL       = regexp.MustCompile(`\bhttps?://[^\s<>()\[\]{}"'` + "`" + `]+`)
	reEmailCI   = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

	reWS  = regexp.MustCompile(`[ \t]+`)
	reCtl = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]+`)

	reCstr = regexp.MustCompile(`(?i)\b(don't|do not|dont|not|never|without|except|only|must|avoid|no change|nochange|keep|preserve|remove)\b`)
)

type intentRule struct {
	label   string
	needAny []string
	needAll []string
	boostRe []*regexp.Regexp
}

var intents = []intentRule{
	{
		label:   "landing",
		needAny: []string{"landing", "landingpage", "lp", "homepage", "home"},
		boostRe: []*regexp.Regexp{regexp.MustCompile(`(?i)\b(institutional|company|corporate|business)\s+(site|website)\b`)},
	},
	{
		label:   "site",
		needAny: []string{"website", "site", "webapp", "web app"},
	},
	{
		label:   "api",
		needAny: []string{"api", "endpoint", "rest", "graphql"},
	},
	{
		label:   "docs",
		needAny: []string{"docs", "documentation", "readme", "guide", "prd", "spec"},
	},
	{
		label:   "cicd",
		needAny: []string{"ci", "cd", "cicd", "pipeline", "github actions", "gitlab ci"},
	},
	{
		label:   "infra",
		needAny: []string{"kubernetes", "k8s", "terraform", "cloudflare", "aws", "gcp", "azure", "docker"},
	},
	{
		label:   "bugfix",
		needAny: []string{"bug", "fix", "error", "issue", "broken", "crash"},
	},
	{
		label:   "refactor",
		needAny: []string{"refactor", "cleanup", "restructure", "improve code"},
	},
}

var stackTags = []struct {
	tag     string
	matches []string
}{
	{"react", []string{"react", "next.js", "nextjs"}},
	{"vue", []string{"vue", "nuxt"}},
	{"svelte", []string{"svelte", "sveltekit"}},
	{"angular", []string{"angular"}},
	{"node", []string{"node", "nodejs", "express", "nestjs"}},
	{"go", []string{"golang", "go "}},
	{"python", []string{"python", "fastapi", "flask", "django"}},
	{"rails", []string{"rails", "ruby on rails"}},
	{"laravel", []string{"laravel", "php"}},
}

var qualifiers = []struct {
	out     string
	matches []string
}{
	{"institutional", []string{"institutional", "corporate", "company", "business"}},
	{"ecommerce", []string{"e-commerce", "ecommerce", "shop", "store"}},
	{"blog", []string{"blog", "posts", "markdown"}},
	{"dashboard", []string{"dashboard", "admin"}},
	{"mobile", []string{"mobile", "android", "ios", "react native"}},
}

func compressIntent(input, mode string, minScore int) string {
	s, _ := protectSpans(input)

	s = normalize(s)

	stacks := detectStacks(s)
	intentLabels, score := detectIntents(s)

	if mode == "safe" {
		if score < minScore+1 || (len(stacks) == 0 && len(intentLabels) == 0) {
			return ""
		}
	} else {
		if score < minScore || (len(stacks) == 0 && len(intentLabels) == 0) {
			return ""
		}
	}

	qs := detectQualifiers(s, 2)

	cstr := ""
	if reCstr.MatchString(s) {
		cstr = " cstr"
	}

	var head []string
	head = append(head, stacks...)
	head = append(head, intentLabels...)

	if len(head) == 0 {
		return ""
	}

	out := strings.Join(uniqueKeepOrder(head), " ")

	if len(qs) > 0 {
		out = out + ": " + strings.Join(qs, ",")
	}
	out += cstr

	return strings.TrimSpace(out)
}

func normalize(s string) string {
	s = reCtl.ReplaceAllString(s, "")
	s = strings.ToLower(s)

	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false

	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		case r == '.' || r == '-' || r == '_' || r == '/' || r == '+':
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}

	return strings.TrimSpace(reWS.ReplaceAllString(b.String(), " "))
}

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

	s = apply(reFenceCode, s)
	s = apply(reInline, s)
	s = apply(reURL, s)
	s = apply(reEmailCI, s)
	return s, items
}

func detectStacks(s string) []string {
	var out []string
	padded := " " + s + " "
	for _, st := range stackTags {
		for _, m := range st.matches {
			mm := " " + strings.TrimSpace(strings.ToLower(m)) + " "
			if strings.Contains(padded, mm) {
				out = append(out, st.tag)
				break
			}
		}
	}
	out = uniqueKeepOrder(out)
	if len(out) > 2 {
		out = out[:2]
	}
	return out
}

func detectIntents(s string) ([]string, int) {
	type scored struct {
		label string
		score int
	}
	padded := " " + s + " "
	var ss []scored

	for _, it := range intents {
		score := 0
		for _, k := range it.needAny {
			if strings.Contains(padded, " "+strings.ToLower(k)+" ") {
				score++
			}
		}
		allOK := true
		for _, k := range it.needAll {
			if !strings.Contains(padded, " "+strings.ToLower(k)+" ") {
				allOK = false
				break
			}
		}
		if allOK && len(it.needAll) > 0 {
			score += 2 * len(it.needAll)
		}
		for _, r := range it.boostRe {
			if r.MatchString(s) {
				score += 2
			}
		}
		if score > 0 {
			ss = append(ss, scored{label: it.label, score: score})
		}
	}

	sort.Slice(ss, func(i, j int) bool {
		if ss[i].score == ss[j].score {
			return ss[i].label < ss[j].label
		}
		return ss[i].score > ss[j].score
	})

	out := make([]string, 0, 2)
	total := 0
	for i := 0; i < len(ss) && i < 2; i++ {
		out = append(out, ss[i].label)
		total += ss[i].score
	}
	return uniqueKeepOrder(out), total
}

func detectQualifiers(s string, limit int) []string {
	var out []string
	padded := " " + s + " "
	for _, q := range qualifiers {
		for _, m := range q.matches {
			if strings.Contains(padded, " "+strings.ToLower(m)+" ") {
				out = append(out, q.out)
				break
			}
		}
	}
	out = uniqueKeepOrder(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func uniqueKeepOrder(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

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
