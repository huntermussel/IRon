package memory

import (
	"strings"
	"testing"
)

func TestKeyPointStore_UpsertAndDSL(t *testing.T) {
	s := NewKeyPointStore()

	s.Upsert(KeyPoint{Type: KPFact, Key: "lang", Value: "go"})
	s.Upsert(KeyPoint{Type: KPFact, Key: "db", Value: "postgres"})
	s.Upsert(KeyPoint{Type: KPPref, Key: "no_frameworks"})
	s.Upsert(KeyPoint{Type: KPTask, Key: "api_server", Value: "done"})

	dsl := s.FormatDSL("default")
	if dsl == "" {
		t.Fatal("expected non-empty DSL")
	}

	// Must contain all four entries
	for _, want := range []string{"lang=go", "db=postgres", "no_frameworks", "api_server:done"} {
		if !strings.Contains(dsl, want) {
			t.Errorf("DSL missing %q, got: %s", want, dsl)
		}
	}
}

func TestKeyPointStore_Upsert_UpdatesExisting(t *testing.T) {
	s := NewKeyPointStore()

	s.Upsert(KeyPoint{Type: KPTask, Key: "tests", Value: "wip"})
	s.Upsert(KeyPoint{Type: KPTask, Key: "tests", Value: "done"}) // update

	pts := s.All("default")
	count := 0
	for _, p := range pts {
		if p.Key == "tests" {
			count++
			if p.Value != "done" {
				t.Errorf("expected value=done, got %q", p.Value)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for 'tests', got %d", count)
	}
}

func TestExtract_Prefs(t *testing.T) {
	kps := Extract("s1", "I prefer Go and I always use stdlib")
	found := false
	for _, k := range kps {
		if k.Type == KPPref {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one KPPref from Extract")
	}
}

func TestExtract_Facts(t *testing.T) {
	kps := Extract("s1", "my language is Go and my database is postgres")
	if len(kps) == 0 {
		t.Fatal("expected key points from fact pattern")
	}
	found := false
	for _, k := range kps {
		if k.Type == KPFact && k.Key == "language" {
			found = true
		}
	}
	if !found {
		t.Error("expected KPFact with key='language'")
	}
}

func TestExtract_Tasks(t *testing.T) {
	kps := Extract("s1", "auth module is done, working on rate limiting")
	tasks := map[string]string{}
	for _, k := range kps {
		if k.Type == KPTask {
			tasks[k.Key] = k.Value
		}
	}
	if tasks["auth_module"] != "done" {
		t.Errorf("expected auth_module:done, got %v", tasks)
	}
	if tasks["rate_limiting"] != "wip" {
		t.Errorf("expected rate_limiting:wip, got %v", tasks)
	}
}

func TestExtract_Rules(t *testing.T) {
	kps := Extract("s1", "don't use external libraries")
	found := false
	for _, k := range kps {
		if k.Type == KPPref && strings.HasPrefix(k.Key, "no_") {
			found = true
		}
	}
	if !found {
		t.Error("expected a no_ preference from 'don't use' pattern")
	}
}

func TestKeyPoint_DSL(t *testing.T) {
	cases := []struct {
		kp   KeyPoint
		want string
	}{
		{KeyPoint{Type: KPFact, Key: "lang", Value: "go"}, "lang=go"},
		{KeyPoint{Type: KPFact, Key: "port"}, "port"},
		{KeyPoint{Type: KPPref, Key: "no_frameworks"}, "no_frameworks"},
		{KeyPoint{Type: KPTask, Key: "api", Value: "done"}, "api:done"},
		{KeyPoint{Type: KPTask, Key: "auth"}, "auth:wip"},
		{KeyPoint{Type: KPTopic, Key: "caching"}, "caching"},
	}
	for _, c := range cases {
		if got := c.kp.DSL(); got != c.want {
			t.Errorf("KeyPoint%+v.DSL() = %q, want %q", c.kp, got, c.want)
		}
	}
}
