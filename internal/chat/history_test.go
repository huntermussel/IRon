package chat

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── FileHistoryStore ─────────────────────────────────────────────────────────

func TestFileHistoryStore_AppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileHistoryStore(dir)
	if err != nil {
		t.Fatalf("NewFileHistoryStore: %v", err)
	}

	msgs := []Message{
		{Role: RoleUser, Content: "hello"},
		{Role: RoleAssistant, Content: "hi there"},
	}
	if err := store.Append("s1", msgs...); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := store.Load("s1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Content != "hello" || got[1].Content != "hi there" {
		t.Errorf("unexpected message content: %+v", got)
	}
}

func TestFileHistoryStore_Load_Missing(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	msgs, err := store.Load("nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for missing session, got: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil messages for missing session, got %v", msgs)
	}
}

func TestFileHistoryStore_Load_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)
	os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("not json {{"), 0600)

	msgs, err := store.Load("corrupt")
	if err != nil {
		t.Fatalf("corrupt file should return nil error (graceful), got: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil messages for corrupt file, got %v", msgs)
	}
}

func TestFileHistoryStore_Pruning(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	// Append more than maxStoredMessages
	for i := 0; i < maxStoredMessages+50; i++ {
		store.Append("prune", Message{Role: RoleUser, Content: "msg"})
	}

	msgs, err := store.Load("prune")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(msgs) > maxStoredMessages {
		t.Errorf("expected at most %d stored messages, got %d", maxStoredMessages, len(msgs))
	}
}

func TestFileHistoryStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	store.Append("del", Message{Role: RoleUser, Content: "bye"})

	if err := store.Delete("del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	msgs, _ := store.Load("del")
	if msgs != nil {
		t.Errorf("expected nil after Delete, got %v", msgs)
	}
}

func TestFileHistoryStore_Delete_Idempotent(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	// Delete a session that never existed — should not error
	if err := store.Delete("ghost"); err != nil {
		t.Errorf("expected no error deleting nonexistent session, got: %v", err)
	}
}

func TestFileHistoryStore_Sessions(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	store.Append("alpha", Message{Role: RoleUser, Content: "a"})
	store.Append("beta", Message{Role: RoleUser, Content: "b"})
	store.Append("gamma", Message{Role: RoleUser, Content: "c"})

	ids, err := store.Sessions()
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 sessions, got %d: %v", len(ids), ids)
	}
}

func TestFileHistoryStore_Sessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	ids, err := store.Sessions()
	if err != nil {
		t.Fatalf("Sessions on empty dir: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(ids))
	}
}

func TestFileHistoryStore_AtomicWrite_NoTmp(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	store.Append("atomic", Message{Role: RoleUser, Content: "x"})

	if _, err := os.Stat(filepath.Join(dir, "atomic.json.tmp")); !os.IsNotExist(err) {
		t.Error(".tmp file should not exist after successful write")
	}
}

func TestSanitizeSessionID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"simple", "simple"},
		{"sess-abc1234", "sess-abc1234"},
		{"sess/../../etc", "sess_.._.._etc"},
		{"", "default"},
		{"hello world", "hello_world"},
	}
	for _, c := range cases {
		got := sanitizeSessionID(c.in)
		if got != c.want {
			t.Errorf("sanitizeSessionID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── Service integration ──────────────────────────────────────────────────────

func TestService_WithHistoryStore_LoadsOnConstruction(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	// Pre-populate history
	store.Append("test-sess",
		Message{Role: RoleUser, Content: "what is 2+2"},
		Message{Role: RoleAssistant, Content: "4"},
	)

	// Create service with the store — it should load those messages
	svc := NewService(nil, WithHistoryStore(store, "test-sess"))

	svc.mu.Lock()
	n := len(svc.history)
	svc.mu.Unlock()

	if n != 2 {
		t.Errorf("expected 2 preloaded messages, got %d", n)
	}
}

func TestService_WithHistoryStore_LoadsTrimmedTo20(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileHistoryStore(dir)

	// Store 30 messages
	for i := 0; i < 30; i++ {
		role := RoleUser
		if i%2 == 1 {
			role = RoleAssistant
		}
		store.Append("big-sess", Message{Role: role, Content: "msg"})
	}

	svc := NewService(nil, WithHistoryStore(store, "big-sess"))

	svc.mu.Lock()
	n := len(svc.history)
	svc.mu.Unlock()

	if n > 20 {
		t.Errorf("expected at most 20 messages in LLM context window, got %d", n)
	}
}

func TestService_WithoutHistoryStore_Unaffected(t *testing.T) {
	// Creating a service without WithHistoryStore should work exactly as before
	svc := NewService(nil)
	if svc.historyStore != nil {
		t.Error("historyStore should be nil when not set")
	}
	if len(svc.history) != 0 {
		t.Errorf("expected empty history on fresh service, got %d", len(svc.history))
	}
}
