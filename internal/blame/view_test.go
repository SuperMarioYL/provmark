package blame

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/provmark/internal/git"
	"github.com/SuperMarioYL/provmark/internal/provenance"
)

func TestRender_AgentLinesFlaggedBesideGitBlame(t *testing.T) {
	// Within one commit git blame reports as "You": lines 1-2 a human-typed
	// hunk, lines 3-4 an agent-generated hunk. provmark surfaces the split.
	lines := []git.BlameLine{
		{LineNo: 1, Content: "package main", Commit: "aaaaaaa", Author: "You"},
		{LineNo: 2, Content: "func a() {}", Commit: "aaaaaaa", Author: "You"},
		{LineNo: 3, Content: "// agent wrote this", Commit: "aaaaaaa", Author: "You"},
		{LineNo: 4, Content: "// and this too", Commit: "aaaaaaa", Author: "You"},
	}
	stamps := []provenance.Stamp{
		{Region: provenance.Region{File: "main.go", Start: 1, End: 2}, AuthorKind: provenance.AuthorHuman},
		{Region: provenance.Region{File: "main.go", Start: 3, End: 4}, AuthorKind: provenance.AuthorAgent, SessionID: "s1", AgentVendor: "claude-code"},
	}
	out := Render(lines, stamps, "main.go")
	if !strings.Contains(out, "agent") {
		t.Fatalf("output should flag agent lines:\n%s", out)
	}
	kind1, _, _ := LineAgent(stamps, 1, "main.go")
	kind3, sess, vendor := LineAgent(stamps, 3, "main.go")
	if kind1 != "human" {
		t.Errorf("line 1 should be human, got %q", kind1)
	}
	if kind3 != "agent" || sess != "s1" || vendor != "claude-code" {
		t.Errorf("line 3 should be agent/s1/claude-code, got %q/%s/%s", kind3, sess, vendor)
	}
	if !strings.Contains(out, "s1 (claude-code)") {
		t.Fatalf("output should show agent session detail:\n%s", out)
	}
	if !strings.Contains(out, "agent wrote this") {
		t.Fatalf("output should include the agent line content:\n%s", out)
	}
}

// LineAgent is a file-aware wrapper for tests: filters stamps to file then asks
// LineAuthor. In production the cmd passes the already-file-filtered stamps to
// LineAuthor via Render.
func LineAgent(stamps []provenance.Stamp, lineNo int, file string) (kind, session, vendor string) {
	var scoped []provenance.Stamp
	for _, s := range stamps {
		if s.Region.File == file {
			scoped = append(scoped, s)
		}
	}
	return LineAuthor(scoped, lineNo)
}

func TestLineAuthor_UncoveredLineIsUnknown(t *testing.T) {
	// provmark never fabricates attribution: a line with no covering stamp is "-".
	stamps := []provenance.Stamp{{Region: provenance.Region{Start: 5, End: 6}, AuthorKind: provenance.AuthorHuman}}
	kind, _, _ := LineAuthor(stamps, 1)
	if kind != "-" {
		t.Fatalf("uncovered line should be '-', got %q", kind)
	}
}

func TestRender_FiltersByFile(t *testing.T) {
	lines := []git.BlameLine{{LineNo: 1, Content: "x", Commit: "abc1234", Author: "A"}}
	stamps := []provenance.Stamp{
		{Region: provenance.Region{File: "other.go", Start: 1, End: 1}, AuthorKind: provenance.AuthorAgent, SessionID: "s1"},
		{Region: provenance.Region{File: "main.go", Start: 1, End: 1}, AuthorKind: provenance.AuthorHuman},
	}
	out := Render(lines, stamps, "main.go")
	// other.go's agent stamp must not leak into main.go's view.
	if strings.Contains(out, "agent") {
		t.Fatalf("main.go view should not show other.go's agent stamp:\n%s", out)
	}
	if !strings.Contains(out, "human") {
		t.Fatalf("main.go view should show human for line 1:\n%s", out)
	}
}

func TestRender_Empty(t *testing.T) {
	if out := Render(nil, nil, "main.go"); out != "" {
		t.Fatalf("empty input should render empty, got %q", out)
	}
}

func TestRender_AlignedColumns(t *testing.T) {
	lines := []git.BlameLine{{LineNo: 1, Content: "x", Commit: "abc1234", Author: "A"}}
	stamps := []provenance.Stamp{{Region: provenance.Region{File: "main.go", Start: 1, End: 1}, AuthorKind: provenance.AuthorHuman}}
	out := Render(lines, stamps, "main.go")
	rows := strings.Split(strings.TrimSpace(out), "\n")
	if len(rows) < 2 {
		t.Fatalf("expected header + row, got %d rows", len(rows))
	}
	// header and data row should align on the "CONTENT" column start.
	headerCols := strings.Index(rows[0], "CONTENT")
	dataCols := strings.Index(rows[1], "x")
	if headerCols != dataCols {
		t.Fatalf("CONTENT column misaligned: header@%d data@%d\n%s", headerCols, dataCols, out)
	}
}
