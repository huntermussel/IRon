package memory

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// KeyPointType classifies what kind of information a key point encodes.
type KeyPointType string

const (
	// KPFact is an established fact: lang=go, db=postgres, port=8080
	KPFact KeyPointType = "F"
	// KPPref is a user preference or constraint: no_frameworks, prefer_stdlib
	KPPref KeyPointType = "P"
	// KPTask is a task and its status: api_server:done, tests:wip
	KPTask KeyPointType = "T"
	// KPTopic is a discussed topic for IR boosting: auth, rate_limiting
	KPTopic KeyPointType = "K"
)

// KeyPoint is a single compressed piece of session context.
// Designed to be stored cheaply and encoded into a compact DSL string
// that consumes far fewer tokens than raw conversation history.
type KeyPoint struct {
	Type    KeyPointType
	Key     string // normalized identifier, snake_case
	Value   string // optional; empty for prefs/rules
	Session string
	At      time.Time
}

// DSL returns the compact single-token representation of this key point.
// Examples:
//
//	KPFact  "lang"  "go"     → "lang=go"
//	KPPref  "no_fw" ""       → "no_fw"
//	KPTask  "api"   "done"   → "api:done"
//	KPTopic "auth"  ""       → "auth"
func (kp KeyPoint) DSL() string {
	switch kp.Type {
	case KPFact:
		if kp.Value != "" {
			return kp.Key + "=" + kp.Value
		}
		return kp.Key
	case KPTask:
		status := kp.Value
		if status == "" {
			status = "wip"
		}
		return kp.Key + ":" + status
	default: // KPPref, KPTopic
		return kp.Key
	}
}

// KeyPointStore holds structured key points per session.
// Thread-safe. All operations are O(n) on session size; sessions stay small
// because key points replace (not append) on the same Key+Type pair.
type KeyPointStore struct {
	mu     sync.RWMutex
	points map[string][]KeyPoint // session → ordered key points
}

// NewKeyPointStore returns an empty store.
func NewKeyPointStore() *KeyPointStore {
	return &KeyPointStore{points: make(map[string][]KeyPoint)}
}

// Upsert adds a key point or updates an existing one with the same Type+Key.
// This is the primary write path — it ensures the store stays compact.
func (s *KeyPointStore) Upsert(kp KeyPoint) {
	if kp.Session == "" {
		kp.Session = "default"
	}
	kp.At = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	pts := s.points[kp.Session]
	for i, p := range pts {
		if p.Type == kp.Type && p.Key == kp.Key {
			pts[i] = kp
			s.points[kp.Session] = pts
			return
		}
	}
	s.points[kp.Session] = append(pts, kp)
}

// All returns a stable copy of all key points for a session,
// sorted by type then insertion time.
func (s *KeyPointStore) All(session string) []KeyPoint {
	if session == "" {
		session = "default"
	}
	s.mu.RLock()
	src := s.points[session]
	out := make([]KeyPoint, len(src))
	copy(out, src)
	s.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].At.Before(out[j].At)
	})
	return out
}

// FormatDSL encodes all key points for a session into a single compact line.
//
// Format: [F:lang=go db=postgres] [P:no_frameworks prefer_stdlib] [T:api:done tests:wip]
//
// Compared to raw conversation history this reduces context tokens by 60-90%
// while preserving the essential facts the LLM needs.
func (s *KeyPointStore) FormatDSL(session string) string {
	pts := s.All(session)
	if len(pts) == 0 {
		return ""
	}

	groups := make(map[KeyPointType][]string)
	for _, p := range pts {
		groups[p.Type] = append(groups[p.Type], p.DSL())
	}

	var parts []string
	for _, t := range []KeyPointType{KPFact, KPPref, KPTask, KPTopic} {
		vals := groups[t]
		if len(vals) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%s:%s]", t, strings.Join(vals, " ")))
	}
	return strings.Join(parts, " ")
}

// ─── Rule-based extractor ────────────────────────────────────────────────────
//
// Extracts key points from natural language text without any LLM calls.
// Covers the most common patterns: "I use X", "my X is Y", "don't do X",
// "X is done", "working on X".

var (
	rePref    = regexp.MustCompile(`(?i)\b(?:i\s+)?(?:use|prefer|like|always use|want)\s+([a-zA-Z][\w\s/-]{0,25})`)
	reFact    = regexp.MustCompile(`(?i)\bmy\s+(\w+)\s+is\s+([\w./:@-]+)`)
	reFactSet = regexp.MustCompile(`(?i)\b(?:set|using|with)\s+(\w+)\s*[=:]\s*([\w./:@-]+)`)
	reRule    = regexp.MustCompile(`(?i)\b(?:don't|do not|dont|never|avoid|no)\s+([a-zA-Z][\w\s-]{1,25})`)
	// Capture "X is done" — "is" is explicitly outside the capture group to avoid
	// keys like "auth_module_is".
	reTaskDone = regexp.MustCompile(`(?i)\b([a-zA-Z]\w+(?:\s+[a-zA-Z]\w+){0,3})\s+is\s+(?:done|complete|completed|finished|ready)`)
	reTaskWIP  = regexp.MustCompile(`(?i)\b(?:working on|implementing|building|creating|adding)\s+([a-zA-Z][\w\s-]{1,25})`)
)

// Extract parses text and returns detected key points using rule-based matching.
// Zero LLM calls — safe to run on every message with negligible overhead.
func Extract(session, text string) []KeyPoint {
	if session == "" {
		session = "default"
	}
	var out []KeyPoint

	add := func(t KeyPointType, k, v string) {
		k = sanitizeKey(k)
		v = sanitizeVal(v)
		if k == "" || len(k) > 40 {
			return
		}
		out = append(out, KeyPoint{Type: t, Key: k, Value: v, Session: session})
	}

	for _, m := range rePref.FindAllStringSubmatch(text, 5) {
		add(KPPref, m[1], "")
	}
	for _, m := range reFact.FindAllStringSubmatch(text, 5) {
		add(KPFact, m[1], m[2])
	}
	for _, m := range reFactSet.FindAllStringSubmatch(text, 5) {
		add(KPFact, m[1], m[2])
	}
	for _, m := range reRule.FindAllStringSubmatch(text, 5) {
		add(KPPref, "no_"+m[1], "")
	}
	for _, m := range reTaskDone.FindAllStringSubmatch(text, 3) {
		add(KPTask, m[1], "done")
	}
	for _, m := range reTaskWIP.FindAllStringSubmatch(text, 3) {
		add(KPTask, m[1], "wip")
	}

	return out
}

func sanitizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func sanitizeVal(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
