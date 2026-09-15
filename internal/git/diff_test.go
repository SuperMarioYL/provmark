package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseDiff_HunksByNewFileRange(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
index 1234567..89abcde 100644
--- a/main.go
+++ b/main.go
@@ -3,4 +3,7 @@ package main
 func a() {}
-removed
+agent wrote this
+and this too
 func b() {}
@@ -12,3 +15,2 @@ func b() {}
-old
+new
`
	hunks, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	want := []Hunk{
		{File: "main.go", StartLine: 3, EndLine: 9},
		{File: "main.go", StartLine: 15, EndLine: 16},
	}
	if !reflect.DeepEqual(hunks, want) {
		t.Fatalf("got %+v, want %+v", hunks, want)
	}
}

func TestParseDiff_SkipsPurelyDeletedHunks(t *testing.T) {
	// A hunk with new count 0 has no new-file lines to stamp; provmark skips it.
	diff := `diff --git a/gone.go b/gone.go
--- a/gone.go
+++ b/gone.go
@@ -1,2 +0,0 @@
-line one
-line two
`
	hunks, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	if len(hunks) != 0 {
		t.Fatalf("expected no hunks for purely-deleted hunk, got %+v", hunks)
	}
}

func TestParseDiff_UsesNewHeaderPath(t *testing.T) {
	// `+++ b/` is authoritative when the diff --git line would be ambiguous.
	diff := `diff --git a/old.go b/renamed.go
--- a/old.go
+++ b/renamed.go
@@ -1,2 +1,3 @@ package x
 a
 b
+c
`
	hunks, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	if len(hunks) != 1 || hunks[0].File != "renamed.go" {
		t.Fatalf("expected renamed.go, got %+v", hunks)
	}
}

func TestParseDiff_MultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,2 @@ package a
 a
+b
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,1 +1,2 @@ package b
 c
+d
`
	hunks, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	if len(hunks) != 2 || hunks[0].File != "a.go" || hunks[1].File != "b.go" {
		t.Fatalf("expected two files, got %+v", hunks)
	}
}

func TestUntrackedHunks_FullFilePerUntrackedFile(t *testing.T) {
	// Regression (v0.2.0): git diff never lists untracked files, so an
	// agent-written brand-new file was invisible to provmark stamp.
	// UntrackedHunks must surface each untracked file as a full-file hunk.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tracked.go":    "a\n",
		"new.go":        "one\ntwo\n",
		"no-eol.go":     "x\ny", // final line without newline still counts
		"sub/nested.go": "n1\nn2\nn3\n",
		"empty.go":      "",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.go"), []byte("i\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInit(t, dir)
	runGit(t, dir, "add", "tracked.go", ".gitignore")
	runGit(t, dir, "commit", "-m", "init")

	hunks, err := UntrackedHunks(dir)
	if err != nil {
		t.Fatalf("UntrackedHunks: %v", err)
	}
	want := []Hunk{
		{File: "new.go", StartLine: 1, EndLine: 2},
		{File: "no-eol.go", StartLine: 1, EndLine: 2},
		{File: "sub/nested.go", StartLine: 1, EndLine: 3},
	}
	if !reflect.DeepEqual(hunks, want) {
		t.Fatalf("got %+v, want %+v", hunks, want)
	}
}

// gitInit creates a throwaway repo with a committable identity.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDiff_EmptyTreeBaseCoversFreshRepo(t *testing.T) {
	// Regression (v0.2.0): in a repo with no commits, plain `git diff` is
	// worktree-vs-index and shows nothing for a staged file. Diffing against
	// the empty tree shows all tracked content as added.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInit(t, dir)
	runGit(t, dir, "add", "a.go")

	hunks, err := Diff(dir, EmptyTreeSHA)
	if err != nil {
		t.Fatalf("Diff against empty tree: %v", err)
	}
	want := []Hunk{{File: "a.go", StartLine: 1, EndLine: 3}}
	if !reflect.DeepEqual(hunks, want) {
		t.Fatalf("got %+v, want %+v", hunks, want)
	}
}

func TestParseBlame_LineOrderAndFields(t *testing.T) {
	out := []byte(`0123456789abcdef0123456789abcdef01234567 (You 2024-01-01 00:00:00 +0000 1)
author You
author-mail <you@example.com>
summary msg
filename main.go
	first line
9876543210fedcba9876543210fedcba9876543210 (You 2024-01-01 00:00:00 +0000 2)
author You
	second line
`)
	lines, err := ParseBlame(out)
	if err != nil {
		t.Fatalf("ParseBlame: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].LineNo != 1 || lines[0].Content != "first line" || lines[0].Author != "You" || lines[0].Commit != "0123456" {
		t.Fatalf("line 0 wrong: %+v", lines[0])
	}
	if lines[1].LineNo != 2 || lines[1].Content != "second line" {
		t.Fatalf("line 1 wrong: %+v", lines[1])
	}
}
