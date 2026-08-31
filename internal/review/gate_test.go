package review

import (
	"testing"

	"github.com/SuperMarioYL/provmark/internal/provenance"
)

func sampleMap() *provenance.ProvenanceMap {
	return &provenance.ProvenanceMap{
		CommitSHA: "abc1234",
		Stamps: []provenance.Stamp{
			{Region: provenance.Region{File: "main.go", Start: 3, End: 9}, AuthorKind: provenance.AuthorAgent, SessionID: "s1", Reviewed: false},
			{Region: provenance.Region{File: "main.go", Start: 15, End: 16}, AuthorKind: provenance.AuthorHuman},
			{Region: provenance.Region{File: "util.go", Start: 1, End: 3}, AuthorKind: provenance.AuthorAgent, SessionID: "s2", Reviewed: false},
			{Region: provenance.Region{File: "util.go", Start: 1, End: 3}, AuthorKind: provenance.AuthorAgent, SessionID: "s3", Reviewed: true},
		},
	}
}

func TestUnreviewed_OnlyAgentUnreviewed(t *testing.T) {
	out := Unreviewed(sampleMap())
	if len(out) != 2 {
		t.Fatalf("expected 2 unreviewed agent stamps, got %d: %+v", len(out), out)
	}
	if out[0].SessionID != "s1" || out[1].SessionID != "s2" {
		t.Fatalf("unexpected unreviewed stamps: %+v", out)
	}
}

func TestUnreviewed_NilMap(t *testing.T) {
	if out := Unreviewed(nil); out != nil {
		t.Fatalf("nil map should yield nil, got %+v", out)
	}
}

func TestMark_FlipsReviewedAndPersists(t *testing.T) {
	dir := t.TempDir()
	pm := sampleMap()
	if err := provenance.Save(dir, pm); err != nil {
		t.Fatalf("Save: %v", err)
	}
	n, err := Mark(dir, "main.go", 5, 7)
	if err != nil {
		t.Fatalf("Mark: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 marked, got %d", n)
	}
	got, _ := provenance.Load(dir)
	// The main.go agent stamp (3-9) overlaps 5-7 → now reviewed.
	var stillUnreviewed int
	for _, s := range got.Stamps {
		if s.AuthorKind == provenance.AuthorAgent && !s.Reviewed {
			stillUnreviewed++
		}
	}
	if stillUnreviewed != 1 {
		t.Fatalf("expected 1 unreviewed agent stamp left (util.go), got %d", stillUnreviewed)
	}
}

func TestMark_NoMatchIsNoOp(t *testing.T) {
	dir := t.TempDir()
	pm := sampleMap()
	_ = provenance.Save(dir, pm)
	n, err := Mark(dir, "nonexistent.go", 1, 1)
	if err != nil {
		t.Fatalf("Mark no-match should not error: %v", err)
	}
	if n != 0 {
		t.Fatalf("no-match should mark 0, got %d", n)
	}
}

func TestParseRange(t *testing.T) {
	cases := []struct {
		in      string
		file    string
		start   int
		end     int
		wantErr bool
	}{
		{"main.go:3-9", "main.go", 3, 9, false},
		{"main.go:5", "main.go", 5, 5, false},
		{"path/to/x.go:1-100", "path/to/x.go", 1, 100, false},
		{"main.go", "", 0, 0, true},
		{"main.go:0", "", 0, 0, true},
		{"main.go:5-3", "", 0, 0, true},
		{"main.go:abc", "", 0, 0, true},
	}
	for _, c := range cases {
		file, start, end, err := ParseRange(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRange(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRange(%q): unexpected error %v", c.in, err)
			continue
		}
		if file != c.file || start != c.start || end != c.end {
			t.Errorf("ParseRange(%q): got %s/%d/%d, want %s/%d/%d", c.in, file, start, end, c.file, c.start, c.end)
		}
	}
}
