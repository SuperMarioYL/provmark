package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLog writes the given lines as a JSONL session log in a temp file.
func writeLog(t *testing.T, name string, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return p
}

func TestJSONLAdapter_Parse(t *testing.T) {
	log := writeLog(t, "sess.jsonl",
		`{"file":"main.go","start_line":4,"end_line":6,"session_id":"s1"}`,
		`{"file":"util.go","start_line":1,"end_line":3,"session_id":"s1"}`,
	)
	a := jsonlAdapter{vendor: "claude-code"}
	events, err := a.Parse(log)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].File != "main.go" || events[0].StartLine != 4 || events[0].EndLine != 6 {
		t.Fatalf("event 0 wrong: %+v", events[0])
	}
	if events[0].Vendor != "claude-code" {
		t.Fatalf("vendor should default to adapter tag, got %q", events[0].Vendor)
	}
}

func TestJSONLAdapter_ParseSkipsUnrecognised(t *testing.T) {
	// A foreign log line (e.g. a richer transcript message) is skipped, not fatal.
	log := writeLog(t, "sess.jsonl",
		`{"type":"message","text":"thinking"}`,
		`{"file":"a.go","start_line":1,"end_line":2}`,
	)
	a := jsonlAdapter{vendor: "deepseek-coder"}
	events, err := a.Parse(log)
	if err != nil {
		t.Fatalf("Parse should still return recoverable events: %v", err)
	}
	if len(events) != 1 || events[0].File != "a.go" {
		t.Fatalf("expected 1 recovered event, got %+v", events)
	}
}

func TestJSONLAdapter_EmptyLogDegrades(t *testing.T) {
	// schema_unverified: a log with no parseable events yields an error callers
	// treat as "no agent events → all-human", never a panic.
	log := writeLog(t, "sess.jsonl", `{"unrelated":true}`)
	a := jsonlAdapter{vendor: "claude-code"}
	events, err := a.Parse(log)
	if err == nil {
		t.Fatalf("expected a degrade error for an empty event stream")
	}
	if events != nil {
		t.Fatalf("expected nil events on degrade, got %+v", events)
	}
	if !strings.Contains(err.Error(), "human") {
		t.Fatalf("error should explain the all-human fallback, got: %v", err)
	}
}

func TestJSONLAdapter_MissingFile(t *testing.T) {
	a := jsonlAdapter{vendor: "claude-code"}
	if _, err := a.Parse(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatalf("expected error for missing log")
	}
	if _, err := a.Parse(""); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestRegistry_GetDefaultsAndUnknown(t *testing.T) {
	r := NewRegistry()
	a, ok := r.Get("")
	if !ok || a.Name() != "claude-code" {
		t.Fatalf("empty name should default to claude-code, got %v/%v", a, ok)
	}
	if _, ok := r.Get("totally-made-up"); ok {
		t.Fatalf("unknown vendor should return ok=false")
	}
	if len(r.Names()) < 2 {
		t.Fatalf("expected at least 2 registered adapters, got %v", r.Names())
	}
}
