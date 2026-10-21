package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests lock the plan §3 happy path (stamp → review → mark → blame) by
// driving the real command functions against a throwaway git repo — the layer
// that wires adapters, diff parsing, correlation, review and blame together,
// which v0.1 shipped with zero coverage.

// newTestRepo creates a committed one-file repo and returns its path.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "a\nb\nc\n")
	writeFile(t, dir, "b.go", "x\ny\n")
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeEvents writes a claude-code JSONL session log into its own temp dir
// (outside the repo under test) covering the given edits, so untracked-file
// stamping sees only the fixture's intended changes.
func writeEvents(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runStampCmd(t *testing.T, dir, log string) (stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	if err := runStamp(dir, log, "claude-code", &out, &errBuf); err != nil {
		t.Fatalf("runStamp: %v\nstderr: %s", err, errBuf.String())
	}
	return out.String(), errBuf.String()
}

func runReviewCmd(t *testing.T, dir string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runReview(dir, false, &out); err != nil {
		t.Fatalf("runReview: %v", err)
	}
	return out.String()
}

func TestStampReviewMarkBlame_HappyPath(t *testing.T) {
	dir := newTestRepo(t)
	// Agent edits a.go line 2; the human leaves b.go untouched.
	writeFile(t, dir, "a.go", "a\nAGENT-EDIT\nc\n")
	log := writeEvents(t,
		`{"file":"a.go","start_line":2,"end_line":2,"session_id":"s1"}`)

	stdout, _ := runStampCmd(t, dir, log)
	if !strings.Contains(stdout, "stamped 1 hunk(s)") {
		t.Fatalf("stamp summary should report the stamped hunk:\n%s", stdout)
	}

	review := runReviewCmd(t, dir)
	if !strings.Contains(review, "1 unreviewed agent region(s)") ||
		!strings.Contains(review, "a.go:") || !strings.Contains(review, "claude-code s1") {
		t.Fatalf("review should list the a.go agent region with session detail:\n%s", review)
	}

	var markOut bytes.Buffer
	if err := runMark(dir, "a.go:2-2", &markOut); err != nil {
		t.Fatalf("runMark: %v", err)
	}
	if !strings.Contains(markOut.String(), "marked 1") {
		t.Fatalf("mark output should report 1 marked:\n%s", markOut.String())
	}
	if review := runReviewCmd(t, dir); !strings.Contains(review, "no unreviewed agent regions") {
		t.Fatalf("review after mark should be clean:\n%s", review)
	}

	var blameOut bytes.Buffer
	if err := runBlame(dir, "a.go", &blameOut); err != nil {
		t.Fatalf("runBlame: %v", err)
	}
	if !strings.Contains(blameOut.String(), "agent: s1 (claude-code)") {
		t.Fatalf("blame should attribute the agent line with session detail:\n%s", blameOut.String())
	}
}

func TestStamp_RetainsCommittedRegionsAcrossRestamp(t *testing.T) {
	// Regression (v0.2.0): re-stamping used to replace the sidecar, so the
	// committed a.go agent region vanished and blame regressed to "-".
	dir := newTestRepo(t)
	writeFile(t, dir, "a.go", "a\nAGENT-ONE\nc\n")
	log1 := writeEvents(t, `{"file":"a.go","start_line":2,"end_line":2,"session_id":"s1"}`)
	runStampCmd(t, dir, log1)
	var markOut bytes.Buffer
	if err := runMark(dir, "a.go:2-2", &markOut); err != nil {
		t.Fatalf("runMark: %v", err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "c1")

	// A second agent session edits the other tracked file; re-stamp.
	writeFile(t, dir, "b.go", "x\nAGENT-TWO\n")
	log2 := writeEvents(t, `{"file":"b.go","start_line":2,"end_line":2,"session_id":"s2"}`)
	stdout, _ := runStampCmd(t, dir, log2)
	if !strings.Contains(stdout, "2 region(s) recorded") {
		t.Fatalf("re-stamp should retain the prior region in the sidecar:\n%s", stdout)
	}

	var blameOut bytes.Buffer
	if err := runBlame(dir, "a.go", &blameOut); err != nil {
		t.Fatalf("runBlame: %v", err)
	}
	if !strings.Contains(blameOut.String(), "agent: s1 (claude-code)") {
		t.Fatalf("blame must keep attribution for the committed agent region:\n%s", blameOut.String())
	}
}

func TestStamp_CoversUntrackedNewFile(t *testing.T) {
	// Regression (v0.2.0): a brand-new untracked file never appears in
	// `git diff`, so an agent-authored file stamped as "no uncommitted hunks"
	// and the review gate reported "clean to commit".
	dir := newTestRepo(t)
	writeFile(t, dir, "newfile.go", "// agent wrote this file\n// line two\n")
	log := writeEvents(t, `{"file":"newfile.go","start_line":1,"end_line":2,"session_id":"s1"}`)

	stdout, stderr := runStampCmd(t, dir, log)
	if strings.Contains(stderr, "no uncommitted hunks") {
		t.Fatalf("untracked new file must be stampable:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	review := runReviewCmd(t, dir)
	if !strings.Contains(review, "newfile.go:1-2") {
		t.Fatalf("review must list the new file's agent region:\n%s", review)
	}
}

func TestStamp_FreshRepoStagedFile(t *testing.T) {
	// Regression (v0.2.0): with no commits yet, the code used to run plain
	// `git diff` (worktree vs index), which shows nothing for a staged file.
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "a\nb\nc\n")
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "add", "a.go")
	log := writeEvents(t, `{"file":"a.go","start_line":1,"end_line":3,"session_id":"s1"}`)

	stdout, stderr := runStampCmd(t, dir, log)
	if strings.Contains(stderr, "no uncommitted hunks") {
		t.Fatalf("a staged file in a fresh repo must be stampable:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	review := runReviewCmd(t, dir)
	if !strings.Contains(review, "a.go:1-3") {
		t.Fatalf("review must list the staged file's agent region:\n%s", review)
	}
}

func TestStamp_NoPhantomAgentRegionAfterHumanRewrite(t *testing.T) {
	// Regression (v0.3.0): after a human rewrote the line an agent had authored,
	// blame reported the line human while review still listed the stale agent
	// region as unreviewed — the review gate flagged a region with no agent lines
	// left. The two views of the sidecar must agree.
	dir := newTestRepo(t)
	writeFile(t, dir, "a.go", "a\nAGENT\nc\n")
	log := writeEvents(t, `{"file":"a.go","start_line":2,"end_line":2,"session_id":"s1"}`)
	runStampCmd(t, dir, log)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "c1")

	// The human rewrites that line themselves; no agent session log this time.
	writeFile(t, dir, "a.go", "a\nHUMAN\nc\n")
	runStampCmd(t, dir, "")

	var blameOut bytes.Buffer
	if err := runBlame(dir, "a.go", &blameOut); err != nil {
		t.Fatalf("runBlame: %v", err)
	}
	if !strings.Contains(blameOut.String(), "human") {
		t.Fatalf("blame should report the rewritten line human:\n%s", blameOut.String())
	}
	if review := runReviewCmd(t, dir); !strings.Contains(review, "no unreviewed agent regions") {
		t.Fatalf("review must agree with blame and be clean:\n%s", review)
	}
}

func TestStamp_TrackedSidecarNotStampedAsEdit(t *testing.T) {
	// Regression (v0.3.0): the sidecar was excluded only from the untracked path.
	// Once .provmark.json was committed and then modified by an earlier stamp,
	// `git diff HEAD` listed it as an ordinary changed file and the next stamp
	// recorded the sidecar's own lines as edit regions.
	dir := newTestRepo(t)
	writeFile(t, dir, "a.go", "a\nAGENT\nc\n")
	log1 := writeEvents(t, `{"file":"a.go","start_line":2,"end_line":2,"session_id":"s1"}`)
	runStampCmd(t, dir, log1)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "c1") // the sidecar is now tracked

	writeFile(t, dir, "a.go", "a\nAGENT\nAGENT2\n")
	log2 := writeEvents(t, `{"file":"a.go","start_line":3,"end_line":3,"session_id":"s2"}`)
	runStampCmd(t, dir, log2) // writes a new sidecar, now dirty against HEAD
	runStampCmd(t, dir, log2) // third stamp: the dirty sidecar appears in git diff HEAD

	data, err := os.ReadFile(filepath.Join(dir, ".provmark.json"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if strings.Contains(string(data), `"file": ".provmark.json"`) {
		t.Fatalf("the sidecar must never record itself as an edit:\n%s", data)
	}
}

// buildProvmark compiles the command under test. The pre-commit hook shells out
// to the real binary, so the hook test cannot run without one.
func buildProvmark(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "provmark")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build provmark: %v\n%s", err, out)
	}
	return bin
}

// runHook executes the installed pre-commit hook the way git would (cwd = repo
// root) and reports its stderr plus whether it exited zero.
func runHook(t *testing.T, dir string) (stderr string, exitOK bool) {
	t.Helper()
	cmd := exec.Command("sh", filepath.Join(dir, ".git", "hooks", "pre-commit"))
	cmd.Dir = dir
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run() // must run before reading errBuf
	return errBuf.String(), err == nil
}

func TestInit_PreCommitHookWarnsAndNeverBlocks(t *testing.T) {
	// Regression (v0.3.0): the installed hook was a hard-coded `exit 0` stub, so
	// committing unreviewed agent code printed nothing even though the plan's m2
	// milestone promises a commit-time warning. The hook must name the unreviewed
	// regions on stderr and still never block the commit.
	dir := newTestRepo(t)
	var initOut bytes.Buffer
	if err := runInit(dir, buildProvmark(t), &initOut); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if strings.Contains(initOut.String(), "m2 scope") {
		t.Fatalf("init must not claim the warning is still m2 scope:\n%s", initOut.String())
	}

	// Clean tree: the hook must stay silent so it does not spam every commit.
	if stderr, ok := runHook(t, dir); !ok || strings.TrimSpace(stderr) != "" {
		t.Fatalf("hook must be silent and non-blocking on a clean tree: exitOK=%v stderr=%q", ok, stderr)
	}

	writeFile(t, dir, "a.go", "a\nAGENT\nc\n")
	log := writeEvents(t, `{"file":"a.go","start_line":2,"end_line":2,"session_id":"s1"}`)
	runStampCmd(t, dir, log)

	stderr, ok := runHook(t, dir)
	if !ok {
		t.Fatalf("hook must never block a commit, but it exited non-zero")
	}
	if !strings.Contains(stderr, "a.go:2-2") {
		t.Fatalf("hook must warn with the unreviewed agent region:\n%s", stderr)
	}
}

func TestReview_QuietSuppressesCleanMessage(t *testing.T) {
	// The pre-commit hook uses --quiet so a clean tree prints nothing.
	dir := newTestRepo(t)
	var out bytes.Buffer
	if err := runReview(dir, true, &out); err != nil {
		t.Fatalf("runReview: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("quiet review on a clean tree must print nothing, got %q", out.String())
	}
}

func TestStamp_SidecarNotStampedAsEdit(t *testing.T) {
	// The sidecar itself is untracked after the first stamp; a re-stamp must
	// not record .provmark.json as an edited region.
	dir := newTestRepo(t)
	writeFile(t, dir, "a.go", "a\nAGENT\nc\n")
	log := writeEvents(t, `{"file":"a.go","start_line":2,"end_line":2,"session_id":"s1"}`)
	runStampCmd(t, dir, log)
	runStampCmd(t, dir, log)

	data, err := os.ReadFile(filepath.Join(dir, ".provmark.json"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if strings.Contains(string(data), `"file": ".provmark.json"`) {
		t.Fatalf("sidecar must not record itself as an edit:\n%s", data)
	}
}
