package memory

import (
	"strings"
	"sync"
	"time"
)

// Procedure is a named, reusable sequence of steps for a recognized task pattern.
//
// Example:
//
//	Name:    "go_build"
//	Pattern: ["go", "build"]
//	Steps:   ["go mod tidy", "go build ./...", "go test ./..."]
//
// When a user query matches all trigger words in Pattern, the procedure's steps
// are injected as a compact DSL hint instead of letting the LLM rediscover them.
//
// Token savings: eliminates repeated "how to build this project" reasoning rounds.
type Procedure struct {
	Name    string    // snake_case identifier
	Pattern []string  // all words must appear in the query (AND semantics)
	Steps   []string  // ordered steps to complete the task
	At      time.Time // last registered/updated
	Hits    int       // usage counter
}

// FormatDSL returns a compact token-efficient representation of the steps.
// Example: "tidy → build → test"
func (p *Procedure) FormatDSL() string {
	compressed := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		// Reduce each step to its most informative tokens (≤ 4 words)
		words := strings.Fields(s)
		if len(words) > 4 {
			words = words[:4]
		}
		compressed = append(compressed, strings.Join(words, "_"))
	}
	return strings.Join(compressed, " → ")
}

// ProcStore caches named procedures and matches them against user queries.
// Thread-safe.
type ProcStore struct {
	mu    sync.RWMutex
	procs map[string]*Procedure // name → procedure
}

// NewProcStore returns an empty store pre-loaded with common Go/dev procedures.
func NewProcStore() *ProcStore {
	s := &ProcStore{procs: make(map[string]*Procedure)}
	s.loadDefaults()
	return s
}

// Register adds or replaces a named procedure.
func (s *ProcStore) Register(p Procedure) {
	p.At = time.Now()
	s.mu.Lock()
	s.procs[p.Name] = &p
	s.mu.Unlock()
}

// Match returns the most specific procedure whose pattern fully matches the query,
// or nil if none match. "Most specific" = longest Pattern slice.
func (s *ProcStore) Match(query string) *Procedure {
	q := strings.ToLower(query)

	s.mu.Lock()
	defer s.mu.Unlock()

	var best *Procedure
	for _, p := range s.procs {
		if matchesAll(q, p.Pattern) {
			if best == nil || len(p.Pattern) > len(best.Pattern) {
				best = p
			}
		}
	}
	if best != nil {
		best.Hits++
	}
	return best
}

// All returns a snapshot of all registered procedures.
func (s *ProcStore) All() []Procedure {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Procedure, 0, len(s.procs))
	for _, p := range s.procs {
		out = append(out, *p)
	}
	return out
}

func matchesAll(query string, pattern []string) bool {
	for _, word := range pattern {
		if !strings.Contains(query, strings.ToLower(word)) {
			return false
		}
	}
	return len(pattern) > 0
}

// loadDefaults seeds the store with common development task procedures.
// These represent the "procedural knowledge" layer that saves the LLM from
// rediscovering standard workflows on every request.
func (s *ProcStore) loadDefaults() {
	defaults := []Procedure{
		{
			Name:    "go_build",
			Pattern: []string{"build", "go"},
			Steps: []string{
				"go mod tidy",
				"go vet ./...",
				"go build ./...",
			},
		},
		{
			Name:    "go_test",
			Pattern: []string{"test", "go"},
			Steps: []string{
				"go test -race ./...",
				"go test -cover ./...",
			},
		},
		{
			Name:    "go_release",
			Pattern: []string{"release", "go"},
			Steps: []string{
				"go mod tidy",
				"go test -race ./...",
				"go build -o bin/ ./...",
				"git tag vX.Y.Z",
				"git push --tags",
			},
		},
		{
			Name:    "docker_build",
			Pattern: []string{"docker", "build"},
			Steps: []string{
				"docker build -t name:tag .",
				"docker run --rm name:tag",
			},
		},
		{
			Name:    "git_pr",
			Pattern: []string{"pull request", "pr"},
			Steps: []string{
				"git checkout -b feat/name",
				"git add -p",
				"git commit -m 'feat: ...'",
				"git push -u origin feat/name",
				"gh pr create",
			},
		},
		{
			Name:    "npm_build",
			Pattern: []string{"npm", "build"},
			Steps: []string{
				"npm install",
				"npm run lint",
				"npm run build",
			},
		},
		{
			Name:    "debug_go",
			Pattern: []string{"debug", "go"},
			Steps: []string{
				"go run -race ./...",
				"go test -v -run TestName ./...",
				"dlv debug ./cmd/...",
			},
		},
	}
	for _, p := range defaults {
		p.At = time.Now()
		s.procs[p.Name] = &p
	}
}
