package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// execCommand is a seam so tests can stub out git invocation if needed. In
// production it is the standard os/exec.Command.
var execCommand = exec.Command

// hookPlaceholder is replaced with the provmark binary path at install time.
const hookPlaceholder = "__PROVMARK_BIN__"

// preCommitScript is the pre-commit hook template: it prints the unreviewed
// agent regions to stderr and always exits 0, so it warns without ever blocking
// a commit (a blocking gate is out of scope for provmark). provmarkPath is baked
// in at install time so the warning works when provmark is not on PATH; the hook
// falls back to a `provmark` on PATH if that binary has since moved.
//
// The hook relies on git's guarantee that it runs from the root of the working
// tree, so `--dir .` is the repository being committed.
const preCommitScript = `#!/usr/bin/env sh
# provmark pre-commit hook — warns on unreviewed agent regions, never blocks.
#
# ` + "`provmark init`" + ` installs this file into .git/hooks/pre-commit. It is
# deliberately non-blocking: the unreviewed agent regions are printed to stderr
# and the hook always exits 0, so a commit is never refused. Silence it by
# reviewing the regions it names:
#
#   provmark review --mark <file>:<start>-<end>

PROVMARK="` + hookPlaceholder + `"
if [ ! -x "$PROVMARK" ]; then
  PROVMARK="$(command -v provmark 2>/dev/null || true)"
fi
if [ -n "$PROVMARK" ]; then
  "$PROVMARK" review --quiet --dir . >&2 || true
fi
exit 0
`

// installPreCommitHook writes the pre-commit hook into dir/.git/hooks/pre-commit
// and makes it executable. provmarkPath is baked into the hook as the binary to
// run. It refuses to overwrite an existing non-provmark hook — printing the path
// and leaving the user to merge manually — so it never silently clobbers a
// custom hook.
func installPreCommitHook(dir, provmarkPath string) (string, error) {
	gitDir, err := gitDirForWorkTree(dir)
	if err != nil {
		return "", err
	}
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return "", fmt.Errorf("create hooks dir: %w", err)
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if existing, err := os.ReadFile(hookPath); err == nil && !isProvmarkHook(string(existing)) {
		return hookPath, fmt.Errorf("refusing to overwrite existing non-provmark pre-commit hook at %s; merge manually", hookPath)
	}
	script := renderPreCommitScript(provmarkPath)
	if err := os.WriteFile(hookPath, []byte(script), 0o755); err != nil {
		return "", fmt.Errorf("write pre-commit hook: %w", err)
	}
	return hookPath, nil
}

// renderPreCommitScript substitutes the provmark binary path into the hook
// template. An empty path leaves the bare name so the hook's `command -v`
// fallback resolves provmark from PATH.
func renderPreCommitScript(provmarkPath string) string {
	if provmarkPath == "" {
		provmarkPath = "provmark"
	}
	return strings.Replace(preCommitScript, hookPlaceholder, provmarkPath, 1)
}

// gitDirForWorkTree resolves the .git dir for a working tree, handling linked
// worktrees where .git is a file pointing elsewhere.
func gitDirForWorkTree(dir string) (string, error) {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", fmt.Errorf("not a git work tree root: %w", err)
	}
	if info.IsDir() {
		return gitPath, nil
	}
	// linked worktree: .git is a file "gitdir: <path>"
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}
	link := string(data)
	const prefix = "gitdir:"
	if len(link) > len(prefix) && link[:len(prefix)] == prefix {
		return filepath.Clean(link[len(prefix):]), nil
	}
	return "", fmt.Errorf("unrecognised .git file at %s", gitPath)
}

// isProvmarkHook reports whether an existing hook content is provmark's own
// stub (so reinstall is safe) versus a user-authored hook (never clobber).
func isProvmarkHook(content string) bool {
	return strings.Contains(content, "provmark pre-commit hook")
}
