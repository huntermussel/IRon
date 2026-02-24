package memory

import (
	"sort"
	"strings"
	"sync"
)

// Store is a simple in-memory KV + lexical scorer to retrieve short snippets
// for token-efficient context.
type Store struct {
	mu   sync.RWMutex
	docs map[string][]string // session -> docs
}

func NewStore() *Store {
	return &Store{docs: make(map[string][]string)}
}

// Index adds a document under a session key.
func (s *Store) Index(session, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[session] = append(s.docs[session], strings.TrimSpace(text))
}

// Query returns up to k snippets ranked by a simple token overlap score.
func (s *Store) Query(session, query string, k int) []string {
	s.mu.RLock()
	docs := s.docs[session]
	s.mu.RUnlock()
	if len(docs) == 0 || strings.TrimSpace(query) == "" || k <= 0 {
		return nil
	}

	qset := tokenSet(query)
	type scored struct {
		text  string
		score int
	}
	var sc []scored
	for _, d := range docs {
		if d == "" {
			continue
		}
		score := overlap(qset, tokenSet(d))
		if score > 0 {
			sc = append(sc, scored{text: d, score: score})
		}
	}
	if len(sc) == 0 {
		return nil
	}
	sort.Slice(sc, func(i, j int) bool {
		if sc[i].score == sc[j].score {
			return len(sc[i].text) < len(sc[j].text)
		}
		return sc[i].score > sc[j].score
	})
	if len(sc) > k {
		sc = sc[:k]
	}
	out := make([]string, 0, len(sc))
	for _, s := range sc {
		out = append(out, s.text)
	}
	return out
}

func tokenSet(s string) map[string]struct{} {
	parts := strings.Fields(strings.ToLower(s))
	set := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, ".,;:!?()[]{}\"'")
		if len(p) < 2 {
			continue
		}
		set[p] = struct{}{}
	}
	return set
}

func overlap(a, b map[string]struct{}) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	count := 0
	for k := range a {
		if _, ok := b[k]; ok {
			count++
		}
	}
	return count
}
