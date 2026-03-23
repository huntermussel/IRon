package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── PersistentKeyPointStore ──────────────────────────────────────────────────

func TestPersistentKPS_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kps.json")

	// Write some key points
	s1 := NewPersistentKeyPointStore(path)
	s1.Upsert(KeyPoint{Type: KPFact, Key: "lang", Value: "go", Session: "s1"})
	s1.Upsert(KeyPoint{Type: KPPref, Key: "no_frameworks", Session: "s1"})
	s1.Upsert(KeyPoint{Type: KPTask, Key: "api", Value: "done", Session: "s1"})

	// Force flush before closing
	s1.Close()

	// Verify file was written
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected keypoints.json to be written")
	}

	// Load into a new store and verify
	s2 := NewPersistentKeyPointStore(path)
	defer s2.Close()

	dsl := s2.FormatDSL("s1")
	for _, want := range []string{"lang=go", "no_frameworks", "api:done"} {
		if !containsStr(dsl, want) {
			t.Errorf("after reload, DSL missing %q; got: %s", want, dsl)
		}
	}
}

func TestPersistentKPS_Upsert_UpdatesExistingAfterReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kps.json")

	s1 := NewPersistentKeyPointStore(path)
	s1.Upsert(KeyPoint{Type: KPTask, Key: "tests", Value: "wip", Session: "s1"})
	s1.Upsert(KeyPoint{Type: KPTask, Key: "tests", Value: "done", Session: "s1"}) // update
	s1.Close()

	s2 := NewPersistentKeyPointStore(path)
	defer s2.Close()

	pts := s2.All("s1")
	count := 0
	for _, p := range pts {
		if p.Key == "tests" {
			count++
			if p.Value != "done" {
				t.Errorf("expected value=done after reload, got %q", p.Value)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'tests' entry after reload, got %d", count)
	}
}

func TestPersistentKPS_MissingFile_StartsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	s := NewPersistentKeyPointStore(path)
	defer s.Close()

	if len(s.All("default")) != 0 {
		t.Error("expected empty store when file doesn't exist")
	}
}

func TestPersistentKPS_CorruptFile_StartsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")
	os.WriteFile(path, []byte("this is not json {{{"), 0600)

	s := NewPersistentKeyPointStore(path)
	defer s.Close()

	if len(s.All("default")) != 0 {
		t.Error("expected empty store when file is corrupt")
	}
}

func TestPersistentKPS_DebouncedSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kps.json")

	s := NewPersistentKeyPointStore(path)

	// Rapid burst of upserts — should be debounced into a single write
	for i := 0; i < 20; i++ {
		s.Upsert(KeyPoint{Type: KPFact, Key: "item", Value: "val", Session: "s"})
	}

	s.Close() // flush

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file should exist after Close()")
	}
}

func TestPersistentKPS_AtomicWrite_NoTmpLeftover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kps.json")

	s := NewPersistentKeyPointStore(path)
	s.Upsert(KeyPoint{Type: KPFact, Key: "x", Value: "y", Session: "s"})
	s.Close()

	// The .tmp file should be renamed away
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after atomic write")
	}
}

// ─── PersistentProcStore ──────────────────────────────────────────────────────

func TestPersistentProcStore_CustomProc_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "procs.json")

	s1 := NewPersistentProcStore(path)
	s1.Register(Procedure{
		Name:    "deploy_staging",
		Pattern: []string{"deploy", "staging"},
		Steps:   []string{"make build", "scp bin/ server:/opt/app", "systemctl restart app"},
	})

	// Verify file written
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected procedures.json to be written after custom Register")
	}

	// Reload
	s2 := NewPersistentProcStore(path)
	p := s2.Match("deploy to staging server now")
	if p == nil {
		t.Fatal("expected custom procedure to be found after reload")
	}
	if p.Name != "deploy_staging" {
		t.Errorf("expected deploy_staging, got %s", p.Name)
	}
}

func TestPersistentProcStore_DefaultsNotPersisted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "procs.json")

	s1 := NewPersistentProcStore(path)
	// Only register a default — should not trigger a write
	_ = s1

	// File should NOT exist (no custom procs registered)
	time.Sleep(10 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("procedures.json should not be written when only defaults are loaded")
	}
}

func TestPersistentProcStore_DefaultsStillLoadedAfterReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "procs.json")

	// Register a custom proc to trigger a file write
	s1 := NewPersistentProcStore(path)
	s1.Register(Procedure{
		Name:    "my_custom",
		Pattern: []string{"my", "custom", "task"},
		Steps:   []string{"step1"},
	})

	// Reload — defaults should still be available
	s2 := NewPersistentProcStore(path)

	// Built-in defaults must still match
	if p := s2.Match("build my go project"); p == nil {
		t.Error("default go_build procedure should still be available after reload")
	}
	// Custom proc must also be present
	if p := s2.Match("run my custom task now"); p == nil {
		t.Error("custom procedure should be restored after reload")
	}
}

// ─── Interface compliance ─────────────────────────────────────────────────────

func TestKeyPointStorer_InterfaceCompliance(t *testing.T) {
	// Both types must satisfy the interface — verified at compile time.
	var _ KeyPointStorer = NewKeyPointStore()
	var _ KeyPointStorer = NewPersistentKeyPointStore(t.TempDir() + "/kps.json")
}

func TestProcStorer_InterfaceCompliance(t *testing.T) {
	var _ ProcStorer = NewProcStore()
	var _ ProcStorer = NewPersistentProcStore(t.TempDir() + "/procs.json")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
