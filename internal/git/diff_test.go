package git

import (
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
