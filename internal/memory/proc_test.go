package memory

import (
	"strings"
	"testing"
)

func TestProcStore_MatchDefault_GoBuild(t *testing.T) {
	s := NewProcStore()
	p := s.Match("how do I build the go project")
	if p == nil {
		t.Fatal("expected match for 'build go' query")
	}
	if p.Name != "go_build" {
		t.Errorf("expected go_build, got %s", p.Name)
	}
}

func TestProcStore_MatchDefault_GoTest(t *testing.T) {
	s := NewProcStore()
	p := s.Match("run test for this go service")
	if p == nil {
		t.Fatal("expected match for 'test go' query")
	}
	if p.Name != "go_test" {
		t.Errorf("expected go_test, got %s", p.Name)
	}
}

func TestProcStore_NoMatch(t *testing.T) {
	s := NewProcStore()
	p := s.Match("what is the meaning of life")
	if p != nil {
		t.Errorf("expected no match, got %s", p.Name)
	}
}

func TestProcStore_Register_Custom(t *testing.T) {
	s := NewProcStore()
	s.Register(Procedure{
		Name:    "deploy_k8s",
		Pattern: []string{"deploy", "kubernetes"},
		Steps:   []string{"kubectl apply -f k8s/", "kubectl rollout status"},
	})
	p := s.Match("deploy to kubernetes cluster")
	if p == nil {
		t.Fatal("expected match for custom procedure")
	}
	if p.Name != "deploy_k8s" {
		t.Errorf("expected deploy_k8s, got %s", p.Name)
	}
}

func TestProcStore_HitCount(t *testing.T) {
	s := NewProcStore()
	s.Match("build go project")
	s.Match("build go binary")
	procs := s.All()
	for _, p := range procs {
		if p.Name == "go_build" {
			if p.Hits < 2 {
				t.Errorf("expected hits >= 2, got %d", p.Hits)
			}
			return
		}
	}
	t.Error("go_build not found in All()")
}

func TestProcedure_FormatDSL(t *testing.T) {
	p := Procedure{
		Name:  "go_build",
		Steps: []string{"go mod tidy", "go build ./..."},
	}
	got := p.FormatDSL()
	if !strings.Contains(got, "→") {
		t.Errorf("expected arrow separator in DSL, got: %s", got)
	}
	if !strings.Contains(got, "go") {
		t.Errorf("expected 'go' in DSL, got: %s", got)
	}
}
