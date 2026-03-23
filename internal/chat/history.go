// Package chat — persistent history layer.
//
// HistoryStore is the interface for persisting conversation messages.
// FileHistoryStore implements it by writing per-session JSON files to a
// configurable directory (default: ~/.iron/history/).
//
// Design choices:
//   - Per-session files: <dir>/<session_id>.json — easy to inspect/delete
//   - Max 200 messages stored per session (independent of the 20-message
//     LLM context window in service.go)
//   - Atomic writes (temp file + rename) on every append
//   - Load returns the last 20 messages for immediate LLM context, but
//     the full file retains up to maxStored for future reference

package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	maxStoredMessages = 200 // cap per session on disk
)

// HistoryStore persists and retrieves chat messages per session.
type HistoryStore interface {
	// Load returns the stored messages for sessionID (newest last).
	// Returns nil, nil when no history exists yet.
	Load(sessionID string) ([]Message, error)

	// Append adds msgs to the session's history, pruning to maxStoredMessages.
	Append(sessionID string, msgs ...Message) error

	// Sessions returns all known session IDs.
	Sessions() ([]string, error)

	// Delete removes all history for a session.
	Delete(sessionID string) error
}

// ─── FileHistoryStore ─────────────────────────────────────────────────────────

// storedSession is the on-disk JSON schema.
type storedSession struct {
	Version   int       `json:"v"`
	SessionID string    `json:"session_id"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

// FileHistoryStore writes one JSON file per session to dir.
type FileHistoryStore struct {
	dir string
	mu  sync.Mutex // serialise concurrent writes for the same session
}

// NewFileHistoryStore creates a store backed by dir.
// The directory is created (0700) if it doesn't exist.
func NewFileHistoryStore(dir string) (*FileHistoryStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &FileHistoryStore{dir: dir}, nil
}

// Load returns all stored messages for sessionID.
func (s *FileHistoryStore) Load(sessionID string) ([]Message, error) {
	b, err := os.ReadFile(s.sessionPath(sessionID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stored storedSession
	if err := json.Unmarshal(b, &stored); err != nil {
		return nil, nil // treat corrupt file as empty; don't propagate
	}
	return stored.Messages, nil
}

// Append adds msgs to sessionID's history file, pruning to maxStoredMessages.
func (s *FileHistoryStore) Append(sessionID string, msgs ...Message) error {
	if len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.Load(sessionID)
	if err != nil {
		existing = nil // ignore load errors; start fresh
	}

	combined := append(existing, msgs...)
	if len(combined) > maxStoredMessages {
		combined = combined[len(combined)-maxStoredMessages:]
	}

	data := storedSession{
		Version:   1,
		SessionID: sessionID,
		UpdatedAt: time.Now(),
		Messages:  combined,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(s.sessionPath(sessionID), b)
}

// Sessions returns all session IDs found in the history directory.
func (s *FileHistoryStore) Sessions() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasSuffix(name, ".json") {
			ids = append(ids, strings.TrimSuffix(name, ".json"))
		}
	}
	return ids, nil
}

// Delete removes the history file for sessionID.
func (s *FileHistoryStore) Delete(sessionID string) error {
	err := os.Remove(s.sessionPath(sessionID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *FileHistoryStore) sessionPath(id string) string {
	return filepath.Join(s.dir, sanitizeSessionID(id)+".json")
}

// sanitizeSessionID strips path separators and control chars so the ID can be
// used safely as a filename on all platforms.
func sanitizeSessionID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" {
		return "default"
	}
	return s
}

// atomicWriteFile writes data to path via a temp file + rename.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
