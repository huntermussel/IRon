package chat

import "testing"

func TestParseToolArgsJSON(t *testing.T) {
	args := parseToolArgs(`{"time":"07:30","label":"wake"}`)
	if args["time"] != "07:30" {
		t.Fatalf("expected time 07:30, got %#v", args["time"])
	}
	if args["label"] != "wake" {
		t.Fatalf("expected label wake, got %#v", args["label"])
	}
}

func TestParseToolArgsInvalidJSONFallsBackToRaw(t *testing.T) {
	raw := `{"time":`
	args := parseToolArgs(raw)
	if args["raw"] != raw {
		t.Fatalf("expected raw fallback, got %#v", args)
	}
}
