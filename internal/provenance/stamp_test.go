package provenance

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SuperMarioYL/provmark/internal/agent"
	"github.com/SuperMarioYL/provmark/internal/git"
)

func TestCorrelate_AgentOverlapStampsAgent(t *testing.T) {
	hunks := []git.Hunk{
		{File: "main.go", StartLine: 3, EndLine: 9},
		{File: "main.go", StartLine: 15, EndLine: 16},
	}
	events := []agent.EditEvent{
		{File: "main.go", StartLine: 4, EndLine: 6, SessionID: "s1", Vendor: "claude-code"},
	}
	stamps := Correlate(hunks, events)
	if len(stamps) != 2 {
		t.Fatalf("expected 2 stamps, got %d", len(stamps))
	}
	if stamps[0].AuthorKind != AuthorAgent {
		t.Errorf("overlapping hunk should be agent, got %q", stamps[0].AuthorKind)
	}
	if stamps[0].SessionID != "s1" || stamps[0].AgentVendor != "claude-code" {
		t.Errorf("agent stamp should carry session/vendor: %+v", stamps[0])
	}
	if stamps[0].Reviewed != false {
		t.Errorf("agent stamp should enter unreviewed")
	}
	if stamps[1].AuthorKind != AuthorHuman {
		t.Errorf("non-overlapping hunk should be human, got %q", stamps[1].AuthorKind)
	}
	if stamps[1].Reviewed != true {
		t.Errorf("human stamp should be reviewed-by-default")
	}
}

func TestCorrelate_NoEventsAllHuman(t *testing.T) {
	// schema_unverified fallback: no agent events → every hunk human.
	hunks := []git.Hunk{{File: "a.go", StartLine: 1, EndLine: 3}}
	stamps := Correlate(hunks, nil)
	if len(stamps) != 1 || stamps[0].AuthorKind != AuthorHuman {
		t.Fatalf("no events should yield all-human, got %+v", stamps)
	}
}

func TestCorrelate_DifferentFileNoMatch(t *testing.T) {
	hunks := []git.Hunk{{File: "main.go", StartLine: 1, EndLine: 5}}
	events := []agent.EditEvent{{File: "other.go", StartLine: 1, EndLine: 5}}
	stamps := Correlate(hunks, events)
	if stamps[0].AuthorKind != AuthorHuman {
		t.Fatalf("non-matching file should be human, got %q", stamps[0].AuthorKind)
	}
}

func TestCorrelate_NoHunksNil(t *testing.T) {
	if stamps := Correlate(nil, []agent.EditEvent{{File: "x", StartLine: 1, EndLine: 2}}); stamps != nil {
		t.Fatalf("no hunks should return nil, got %+v", stamps)
	}
}

func TestBuild_CommitsSHACarried(t *testing.T) {
	hunks := []git.Hunk{{File: "a.go", StartLine: 1, EndLine: 2}}
	pm := Build("abc1234", hunks, nil)
	if pm.CommitSHA != "abc1234" {
		t.Fatalf("CommitSHA not carried: %q", pm.CommitSHA)
	}
	if len(pm.Stamps) != 1 || pm.Stamps[0].AuthorKind != AuthorHuman {
		t.Fatalf("build should produce one human stamp, got %+v", pm.Stamps)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 31, 1, 0, 0, 0, time.UTC)
	pm := &ProvenanceMap{
		CommitSHA: "abc1234",
		Stamps: []Stamp{
			{Region: Region{File: "main.go", Start: 3, End: 9}, AuthorKind: AuthorAgent, SessionID: "s1", AgentVendor: "claude-code", Reviewed: false, StampedAt: now},
			{Region: Region{File: "main.go", Start: 15, End: 16}, AuthorKind: AuthorHuman, Reviewed: true, StampedAt: now},
		},
	}
	if err := Save(dir, pm); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CommitSHA != "abc1234" {
		t.Fatalf("CommitSHA round-trip: %q", loaded.CommitSHA)
	}
	if !reflect.DeepEqual(loaded.Stamps, pm.Stamps) {
		t.Fatalf("stamps round-trip mismatch:\n got %+v\nwant %+v", loaded.Stamps, pm.Stamps)
	}
}

func TestLoad_MissingSidecarReturnsEmpty(t *testing.T) {
	// Pre-stamp state: no sidecar → empty map, no error.
	pm, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("missing sidecar should not error: %v", err)
	}
	if pm.CommitSHA != "" || len(pm.Stamps) != 0 {
		t.Fatalf("missing sidecar should be empty, got %+v", pm)
	}
}

func TestSave_PreservesReviewStateOnReStamp(t *testing.T) {
	dir := t.TempDir()
	// First stamp: one agent region, unreviewed.
	first := &ProvenanceMap{
		CommitSHA: "v1",
		Stamps: []Stamp{
			{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, SessionID: "s1", Reviewed: false, StampedAt: time.Now()},
		},
	}
	if err := Save(dir, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	// Simulate a review: mark reviewed on disk.
	loaded, _ := Load(dir)
	loaded.Stamps[0].Reviewed = true
	if err := Save(dir, loaded); err != nil {
		t.Fatalf("Save reviewed: %v", err)
	}

	// Re-stamp the same region: the stamp command regenerates fresh stamps
	// (Reviewed=false), then merges prior review state before saving.
	reStamp := &ProvenanceMap{
		CommitSHA: "v2",
		Stamps: []Stamp{
			{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, SessionID: "s1", Reviewed: false, StampedAt: time.Now()},
		},
	}
	prev, _ := Load(dir) // prior sidecar (Reviewed=true)
	MergeReviewState(prev, reStamp)
	if err := Save(dir, reStamp); err != nil {
		t.Fatalf("Save re-stamp: %v", err)
	}
	got, _ := Load(dir)
	if !got.Stamps[0].Reviewed {
		t.Fatalf("re-stamp should preserve Reviewed=true, got %+v", got.Stamps[0])
	}
	if got.CommitSHA != "v2" {
		t.Fatalf("re-stamp should take the new CommitSHA, got %q", got.CommitSHA)
	}
}

func TestMergeReviewState_OnlyMatchingRegions(t *testing.T) {
	prev := &ProvenanceMap{Stamps: []Stamp{
		{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, Reviewed: true},
	}}
	// A new region (4-5) has no prior state → stays unreviewed; 1-3 keeps reviewed.
	next := &ProvenanceMap{Stamps: []Stamp{
		{Region: Region{File: "a.go", Start: 4, End: 5}, AuthorKind: AuthorAgent, Reviewed: false},
		{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, Reviewed: false},
	}}
	MergeReviewState(prev, next)
	if !next.Stamps[1].Reviewed {
		t.Errorf("matching region should keep Reviewed=true")
	}
	if next.Stamps[0].Reviewed {
		t.Errorf("new region should stay unreviewed")
	}
}

func TestStampKey_DistinguishesByAuthorKind(t *testing.T) {
	a := Stamp{Region: Region{File: "f", Start: 1, End: 2}, AuthorKind: AuthorAgent}
	h := Stamp{Region: Region{File: "f", Start: 1, End: 2}, AuthorKind: AuthorHuman}
	if stampKey(a) == stampKey(h) {
		t.Fatalf("agent and human stamps of the same region must have distinct keys")
	}
}

func TestAccumulate_RetainsPriorRegionsNotReStamped(t *testing.T) {
	// Regression (v0.2.0): a re-stamp used to replace the whole sidecar, so a
	// committed agent region vanished and provmark blame regressed to "-".
	// Accumulate must retain prior stamps the fresh diff does not supersede.
	prev := &ProvenanceMap{
		CommitSHA: "v1",
		Stamps: []Stamp{
			{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, SessionID: "s1", AgentVendor: "claude-code", Reviewed: true, StampedAt: time.Now()},
		},
	}
	fresh := &ProvenanceMap{
		CommitSHA: "v2",
		Stamps: []Stamp{
			{Region: Region{File: "b.go", Start: 1, End: 2}, AuthorKind: AuthorAgent, SessionID: "s2", AgentVendor: "claude-code", Reviewed: false, StampedAt: time.Now()},
		},
	}
	got := Accumulate(prev, fresh)
	if got.CommitSHA != "v2" {
		t.Fatalf("accumulated map should take the fresh CommitSHA, got %q", got.CommitSHA)
	}
	if len(got.Stamps) != 2 {
		t.Fatalf("expected fresh + retained prior stamp, got %+v", got.Stamps)
	}
	if got.Stamps[0].Region.File != "b.go" || got.Stamps[1].Region.File != "a.go" {
		t.Fatalf("fresh stamps should come first, then retained: %+v", got.Stamps)
	}
	if !got.Stamps[1].Reviewed || got.Stamps[1].SessionID != "s1" {
		t.Fatalf("retained prior stamp should keep its history: %+v", got.Stamps[1])
	}
}

func TestAccumulate_FreshSupersedesSameKey(t *testing.T) {
	prev := &ProvenanceMap{CommitSHA: "v1", Stamps: []Stamp{
		{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, Reviewed: true, StampedAt: time.Now()},
	}}
	fresh := &ProvenanceMap{CommitSHA: "v2", Stamps: []Stamp{
		{Region: Region{File: "a.go", Start: 1, End: 3}, AuthorKind: AuthorAgent, Reviewed: false, StampedAt: time.Now()},
	}}
	got := Accumulate(prev, fresh)
	if len(got.Stamps) != 1 {
		t.Fatalf("same-key fresh stamp must supersede prior, got %+v", got.Stamps)
	}
}

func TestAccumulate_NilInputs(t *testing.T) {
	pm := &ProvenanceMap{CommitSHA: "x"}
	if got := Accumulate(nil, pm); got != pm {
		t.Fatalf("nil prev should return fresh")
	}
	if got := Accumulate(pm, nil); got != pm {
		t.Fatalf("nil fresh should return prev")
	}
}

func TestSidecarPath(t *testing.T) {
	if filepath.Base(SidecarName) != ".provmark.json" {
		t.Fatalf("unexpected sidecar name %q", SidecarName)
	}
}
