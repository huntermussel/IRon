package trashcleanner

import (
	"strings"
	"testing"
)

func TestCompressEnglishPrompt_PreservesNegationAndTechTokens(t *testing.T) {
	in := "I do NOT want Kubernetes right now; I only need a simple deploy on Cloudflare Pages v2.1.0 with Redis."
	out := compressPragmatic(in, "aggr")

	if !strings.Contains(out, "not") {
		t.Fatalf("expected negation to be preserved, got: %q", out)
	}
	if !strings.Contains(out, "kubernetes") {
		t.Fatalf("expected key noun to remain, got: %q", out)
	}
	if !strings.Contains(out, "Cloudflare") && !strings.Contains(out, "cloudflare") {
		t.Fatalf("expected proper noun to remain, got: %q", out)
	}
	if !strings.Contains(out, "v2.1.0") {
		t.Fatalf("expected version token to remain, got: %q", out)
	}
	if strings.Contains(out, " i ") || strings.HasPrefix(out, "i ") {
		t.Fatalf("expected stopwords to be dropped, got: %q", out)
	}
}
