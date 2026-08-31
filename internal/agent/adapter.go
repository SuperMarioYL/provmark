// Package agent parses vendor-specific agent session logs into a neutral,
// agent-substrate-agnostic edit-event stream. provmark binds at the
// edit-event layer, not the model layer: any coding agent whose edit-event
// stream an adapter can parse is stampable.
//
// NOTE: the real on-disk transcript formats of Claude Code, DeepSeek-Coder and
// Qwen-Coder are UNVERIFIED as of m1 (plan frontmatter: schema_unverified:
// true). The adapters below parse a documented neutral JSONL contract that the
// absorption probe (docs/absorption-probe.md) is meant to reconcile against the
// real formats. Parsing degrades gracefully: an unrecognised or missing log
// yields no events and a warning rather than a hard failure, so `provmark
// stamp` still produces an all-human provenance map when the agent substrate
// can't be read.
package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditEvent is the neutral, vendor-agnostic record of one region an agent edited
// during a session. StartLine and EndLine are 1-based, inclusive, in the
// post-edit version of File — the same coordinate space as git.Hunk, so the
// correlator can match by line overlap without transformation.
type EditEvent struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	SessionID string `json:"session_id,omitempty"`
	Vendor    string `json:"vendor,omitempty"`
}

// Adapter parses a vendor session log into normalized EditEvents. A new agent
// lands as an Adapter implementation registered with Registry; the provenance
// core never branches on vendor.
type Adapter interface {
	// Name is the vendor slug the user passes to --vendor (e.g. "claude-code").
	Name() string
	// Parse reads sessionLogPath and returns the edit events it contains. A
	// missing file or unparseable content is NOT fatal for stamping: return the
	// events that could be recovered plus a non-nil error so callers can warn
	// and fall back to all-human attribution.
	Parse(sessionLogPath string) ([]EditEvent, error)
}

// jsonlAdapter is the shared neutral-JSONL parser. Each adapter is a thin
// vendor-tagged instance; the neutral event schema is the whole point of the
// abstraction — a single on-disk format provmark can stamp regardless of which
// agent wrote it, once an adapter reconciles it to EditEvents.
type jsonlAdapter struct {
	vendor string
}

// Name returns the vendor slug.
func (a jsonlAdapter) Name() string { return a.vendor }

// Parse reads one JSON object per line from sessionLogPath. Lines that do not
// decode as an EditEvent are skipped with a count, so a partially-foreign log
// still yields the events it does contain rather than failing wholesale.
func (a jsonlAdapter) Parse(sessionLogPath string) ([]EditEvent, error) {
	if sessionLogPath == "" {
		return nil, fmt.Errorf("agent: no session log path provided for %q", a.vendor)
	}
	f, err := os.Open(sessionLogPath)
	if err != nil {
		return nil, fmt.Errorf("agent: open %s session log %q: %w", a.vendor, sessionLogPath, err)
	}
	defer f.Close()

	var events []EditEvent
	var skipped int
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev EditEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			skipped++
			continue
		}
		if ev.File == "" || ev.EndLine < ev.StartLine {
			skipped++
			continue
		}
		if ev.Vendor == "" {
			ev.Vendor = a.vendor
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("agent: read %s session log: %w", a.vendor, err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("agent: %s log %q yielded no edit events (%d unrecognised lines); "+
			"stamping all hunks as human", a.vendor, sessionLogPath, skipped)
	}
	return events, nil
}

// Registry maps vendor slugs to adapters. The slugs are the only vendor
// knowledge that escapes the agent package.
type Registry struct {
	byName map[string]Adapter
}

// NewRegistry returns a registry with the v0.1 adapter set. Per the plan, v0.1
// ships Claude Code + one 国产 coding agent (DeepSeek-Coder); Qwen-Coder is
// registered by slug so a future adapter can drop in without touching callers.
func NewRegistry() *Registry {
	r := &Registry{byName: make(map[string]Adapter)}
	r.Register(jsonlAdapter{vendor: "claude-code"})
	r.Register(jsonlAdapter{vendor: "deepseek-coder"})
	r.Register(jsonlAdapter{vendor: "qwen-coder"})
	return r
}

// Register adds (or replaces) an adapter. Future adapters land here.
func (r *Registry) Register(a Adapter) {
	r.byName[a.Name()] = a
}

// Get returns the adapter for name, defaulting to claude-code when name is empty
// (the plan's default adapter choice). Returns ok=false for an unknown vendor
// so the caller can surface a clear error rather than a silent nil.
func (r *Registry) Get(name string) (Adapter, bool) {
	if name == "" {
		name = "claude-code"
	}
	a, ok := r.byName[name]
	return a, ok
}

// Names returns the registered vendor slugs, sorted, for `provmark stamp
// --help` and the init prompt.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	// stable order for help text
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}
