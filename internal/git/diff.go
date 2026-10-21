// Package git parses git output into the structures provmark correlates against.
//
// provmark attributes at edit granularity *within* a commit; git attributes at
// commit granularity (the committer). This package is the git side of that gap:
// it turns `git diff` unified output into per-file hunks (the changed regions)
// and `git blame --line-porcelain` into per-line metadata. It owns no
// provenance concept — only git's view of the working tree.
package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Hunk is a contiguous changed region in the new (post-change) version of a
// file, as reported by `git diff`. Start and End are 1-based line numbers in
// the new file, End inclusive. A hunk with Start==0 means the file was added
// with no lines (e.g. mode-only change); callers should skip such regions.
type Hunk struct {
	File      string
	StartLine int
	EndLine   int
}

// BlameLine is one line of a file annotated with the commit git blames it on.
// LineNo is 1-based in the working tree; Commit is the short SHA; Author is
// the git author name git blame reports.
type BlameLine struct {
	LineNo  int
	Content string
	Commit  string
	Author  string
}

// hunkHeaderRe matches the `@@ -l,s +l,s @@` unified-diff hunk header. Counts
// are optional (a single line is shown as `@@ -l +l @@`).
var hunkHeaderRe = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ParseDiff parses `git diff` unified output into per-file hunks keyed on the
// new-file line ranges. It tracks the current file across `diff --git`,
// `--- a/`, `+++ b/` headers and emits one Hunk per `@@` block whose new-side
// count is greater than zero (purely-deleted hunks have no new lines to stamp).
func ParseDiff(diffText string) ([]Hunk, error) {
	var hunks []Hunk
	file := ""
	scanner := bufio.NewScanner(strings.NewReader(diffText))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file = parseDiffGitPath(line)
		case strings.HasPrefix(line, "+++ "):
			if p := parseHeaderPath(line); p != "" && p != "/dev/null" {
				file = p
			}
		case strings.HasPrefix(line, "@@ "):
			start, end, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			if end < start { // zero new lines in this hunk
				continue
			}
			if file == "" {
				return nil, fmt.Errorf("git: hunk header before file path: %q", line)
			}
			hunks = append(hunks, Hunk{File: file, StartLine: start, EndLine: end})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("git: scan diff: %w", err)
	}
	return hunks, nil
}

// parseDiffGitPath extracts the b/ path from a `diff --git a/x b/x` line. git
// quotes paths containing spaces; the unquoted b-side is what we want.
func parseDiffGitPath(line string) string {
	rest := strings.TrimPrefix(line, "diff --git ")
	// rest = "a/<path> b/<path>" (possibly quoted)
	parts := splitUnquoted(rest)
	if len(parts) < 2 {
		return ""
	}
	return unquotePath(strings.TrimPrefix(parts[1], "b/"))
}

// parseHeaderPath extracts the path from a `+++ b/<path>` (or `+++ /dev/null`)
// line, which is more reliable than the diff --git line when paths are quoted.
func parseHeaderPath(line string) string {
	rest := strings.TrimPrefix(line, "+++ ")
	rest = strings.TrimPrefix(rest, "b/")
	return unquotePath(rest)
}

// parseHunkHeader returns the new-file (1-based, inclusive) range of a hunk.
// ok is false when the line is not a parseable hunk header or the new count is
// zero (no added/ context lines on the new side — nothing to stamp).
func parseHunkHeader(line string) (start, end int, ok bool) {
	m := hunkHeaderRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	start, _ = strconv.Atoi(m[2])
	count := 1
	if m[3] != "" {
		count, _ = strconv.Atoi(m[3])
	}
	if count == 0 {
		return 0, 0, false
	}
	return start, start + count - 1, true
}

// unquotePath reverses git's octal-quoting of paths with spaces/special chars
// (e.g. "\"file with space.go\"" → "file with space.go").
func unquotePath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if unq, err := strconv.Unquote(p); err == nil {
			return unq
		}
	}
	return p
}

// splitUnquoted splits a `git diff --git` argument list on whitespace that is
// not inside double quotes, so `a/foo bar.go b/foo bar.go` splits into two.
func splitUnquoted(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// EmptyTreeSHA is git's well-known empty-tree object hash. Diffing against it
// shows every tracked file as newly added, which is the correct "everything
// changed" base for a repo that has no commits yet — plain `git diff` there
// compares worktree to index and misses staged files entirely.
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Diff runs `git diff` with the given extra args in dir and returns parsed
// hunks. Pass "HEAD" to diff the working tree against HEAD (staged + unstaged),
// which is what `provmark stamp` correlates against.
//
// Zero context (-U0) is deliberate: with git's default 3 lines of context a
// hunk's new-side range includes unchanged lines, and since the correlator
// stamps a whole hunk with the matched author-kind those untouched lines would
// inherit the agent label — a one-line agent edit would report the surrounding
// human-typed lines as agent. -U0 makes a hunk exactly the changed lines.
func Diff(dir string, args ...string) ([]Hunk, error) {
	cmdArgs := append([]string{"-C", dir, "diff", "--no-color", "-U0"}, args...)
	out, err := exec.Command("git", cmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("git: run diff in %q: %w", dir, err)
	}
	return ParseDiff(string(out))
}

// UntrackedHunks returns one full-file hunk (lines 1..N) for every untracked
// file in dir, excluding ignored files. `git diff` never lists untracked
// files, yet a brand-new file is the most common agent output; these hunks
// let the correlator stamp new files like any other changed region. Files
// that are empty or unreadable yield no hunk — there are no lines to stamp.
func UntrackedHunks(dir string) ([]Hunk, error) {
	out, err := exec.Command("git", "-C", dir, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git: list untracked files in %q: %w", dir, err)
	}
	var hunks []Hunk
	for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if name == "" {
			continue
		}
		n, err := lineCount(filepath.Join(dir, name))
		if err != nil || n == 0 {
			continue
		}
		hunks = append(hunks, Hunk{File: name, StartLine: 1, EndLine: n})
	}
	return hunks, nil
}

// lineCount counts lines the way git's diff side does: newline-terminated
// lines, plus one for a final unterminated line.
func lineCount(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++
	}
	return n, nil
}

// HeadSHA returns the short SHA of HEAD in dir, or "" if the repo has no
// commits yet. provmark stamps the working-tree diff against HEAD, so this is
// the diff base recorded in the sidecar's CommitSHA.
func HeadSHA(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Blame runs `git blame --line-porcelain` on file in dir and returns one
// BlameLine per line of the file in working-tree order.
func Blame(dir, file string) ([]BlameLine, error) {
	cmd := exec.Command("git", "-C", dir, "blame", "--line-porcelain", "--", file)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git: blame %q in %q: %w", file, dir, err)
	}
	return ParseBlame(out)
}

// ParseBlame turns `git blame --line-porcelain` output into BlameLines. The
// porcelain format alternates header lines (`<key> <value>`) with a TAB-prefixed
// data line; `author ` and `<short> (` (the commit header) are the fields we keep.
func ParseBlame(out []byte) ([]BlameLine, error) {
	var lines []BlameLine
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNo := 0
	commit := ""
	author := ""
	for scanner.Scan() {
		raw := scanner.Text()
		switch {
		case strings.HasPrefix(raw, "\t"):
			lineNo++
			lines = append(lines, BlameLine{
				LineNo:  lineNo,
				Content: strings.TrimPrefix(raw, "\t"),
				Commit:  commit,
				Author:  author,
			})
			commit, author = "", ""
		case strings.HasPrefix(raw, "author "):
			author = strings.TrimPrefix(raw, "author ")
		case len(raw) >= 40 && raw[40:41] == " " && looksHex(raw[:40]):
			commit = raw[:7] // short sha
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("git: scan blame: %w", err)
	}
	return lines, nil
}

// looksHex reports whether s is a 40-char lowercase hex string (a git SHA-1).
func looksHex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
