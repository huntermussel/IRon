// Package memory — persistence layer.
//
// PersistentKeyPointStore and PersistentProcStore wrap their in-memory
// counterparts with automatic JSON file I/O so that accumulated context
// survives IRon restarts.
//
// Files written:
//   ~/.iron/keypoints.json   — per-session key points (facts, prefs, tasks)
//   ~/.iron/procedures.json  — user-registered custom procedures
//
// Default procedures (hardcoded in proc.go) are NOT persisted — they are
// always re-loaded from code so updating IRon picks up new defaults.
//
// Writes are atomic (temp-file + rename) and debounced at 500ms so rapid
// Upsert bursts (e.g. end-of-turn extraction) produce a single disk write.

package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ─── PersistentKeyPointStore ─────────────────────────────────────────────────

// serializedKPS is the on-disk JSON schema for key points.
type serializedKPS struct {
	Version  int                    `json:"v"`
	Sessions map[string][]KeyPoint  `json:"sessions"`
}

// PersistentKeyPointStore wraps KeyPointStore with automatic JSON persistence.
// It satisfies the KeyPointStorer interface.
type PersistentKeyPointStore struct {
	*KeyPointStore
	path   string
	saveCh chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewPersistentKeyPointStore creates a disk-backed store at path.
// Existing data is loaded synchronously; background saves are debounced at 500ms.
func NewPersistentKeyPointStore(path string) *PersistentKeyPointStore {
	_ = os.MkdirAll(filepath.Dir(path), 0700)

	s := &PersistentKeyPointStore{
		KeyPointStore: NewKeyPointStore(),
		path:          path,
		saveCh:        make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
	s.load()

	s.wg.Add(1)
	go s.saveLoop()
	return s
}

// Upsert stores a key point and schedules a debounced disk write.
func (s *PersistentKeyPointStore) Upsert(kp KeyPoint) {
	s.KeyPointStore.Upsert(kp)
	select {
	case s.saveCh <- struct{}{}:
	default: // save already queued
	}
}

// Close flushes any pending write and stops the background goroutine.
// Call this on shutdown to guarantee data is not lost.
func (s *PersistentKeyPointStore) Close() {
	close(s.done)
	s.wg.Wait()
}

func (s *PersistentKeyPointStore) saveLoop() {
	defer s.wg.Done()
	t := time.NewTimer(0)
	t.Stop()
	for {
		select {
		case <-s.saveCh:
			t.Reset(500 * time.Millisecond)
		case <-t.C:
			s.flush()
		case <-s.done:
			t.Stop()
			s.flush() // final flush before exit
			return
		}
	}
}

func (s *PersistentKeyPointStore) flush() {
	s.KeyPointStore.mu.RLock()
	data := serializedKPS{
		Version:  1,
		Sessions: make(map[string][]KeyPoint, len(s.KeyPointStore.points)),
	}
	for sess, pts := range s.KeyPointStore.points {
		cp := make([]KeyPoint, len(pts))
		copy(cp, pts)
		data.Sessions[sess] = cp
	}
	s.KeyPointStore.mu.RUnlock()

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	atomicWrite(s.path, b)
}

func (s *PersistentKeyPointStore) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return // file not found on first run
	}
	var data serializedKPS
	if err := json.Unmarshal(b, &data); err != nil {
		return // corrupt file — start fresh
	}
	s.KeyPointStore.mu.Lock()
	for sess, pts := range data.Sessions {
		s.KeyPointStore.points[sess] = pts
	}
	s.KeyPointStore.mu.Unlock()
}

// ─── PersistentProcStore ─────────────────────────────────────────────────────

// serializedProcs is the on-disk JSON schema for custom procedures.
type serializedProcs struct {
	Version int         `json:"v"`
	Custom  []Procedure `json:"custom"`
}

// defaultProcNames are the procedure names seeded by loadDefaults() in proc.go.
// These are not persisted — they live in code so new IRon versions can update them.
var defaultProcNames = map[string]bool{
	"go_build": true, "go_test": true, "go_release": true,
	"docker_build": true, "git_pr": true, "npm_build": true, "debug_go": true,
}

// PersistentProcStore wraps ProcStore with JSON persistence for custom (user-registered)
// procedures. It satisfies the ProcStorer interface.
type PersistentProcStore struct {
	*ProcStore
	path string
}

// NewPersistentProcStore creates a disk-backed proc store at path.
// Default procedures from loadDefaults() are always loaded from code.
// User-registered procedures are loaded from path on startup.
func NewPersistentProcStore(path string) *PersistentProcStore {
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	s := &PersistentProcStore{
		ProcStore: NewProcStore(), // loads defaults from code
		path:      path,
	}
	s.load()
	return s
}

// Register adds a procedure and immediately persists it (if it's custom).
func (s *PersistentProcStore) Register(p Procedure) {
	s.ProcStore.Register(p)
	if !defaultProcNames[p.Name] {
		s.save()
	}
}

func (s *PersistentProcStore) save() {
	s.ProcStore.mu.RLock()
	var custom []Procedure
	for _, p := range s.ProcStore.procs {
		if !defaultProcNames[p.Name] {
			custom = append(custom, *p)
		}
	}
	s.ProcStore.mu.RUnlock()

	if len(custom) == 0 {
		return
	}
	data := serializedProcs{Version: 1, Custom: custom}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	atomicWrite(s.path, b)
}

func (s *PersistentProcStore) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var data serializedProcs
	if err := json.Unmarshal(b, &data); err != nil {
		return
	}
	for _, p := range data.Custom {
		s.ProcStore.Register(p)
	}
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

// atomicWrite writes data to path via a temp file + rename, ensuring no
// partially-written files survive a crash or concurrent access.
func atomicWrite(path string, data []byte) {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	os.Rename(tmp, path) //nolint:errcheck
}
