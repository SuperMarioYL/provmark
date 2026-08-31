package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// execCommand is a seam so tests can stub out git invocation if needed. In
// production it is the standard os/exec.Command.
var execCommand = exec.Command

// preCommitScript is the v0.1 (m1) pre-commit hook: a non-blocking stub. The
// real review-gate warning (which would call `provmark review` and surface
// unreviewed agent regions without blocking) is m2 scope — shipped as a stub so
// `provmark init` is wired end-to-end without committing an unfinished gate.
const preCommitScript = `#!/usr/bin/env sh
# provmark pre-commit hook (v0.1 stub)
# m1 scope: this hook is a no-op placeholder. The review-gate warning that
# surfaces unreviewed agent regions without blocking the commit is m2 scope.
# When m2 ships, this hook will run: provmark review && warn on unreviewed
# agent regions (never block).
exit 0
`

// installPreCommitHook writes the pre-commit hook into dir/.git/hooks/pre-commit
// and makes it executable. It refuses to overwrite an existing non-provmark
// hook — printing the path and leaving the user to merge manually — so it never
// silently clobbers a custom hook.
func installPreCommitHook(dir string) (string, error) {
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
	if err := os.WriteFile(hookPath, []byte(preCommitScript), 0o755); err != nil {
		return "", fmt.Errorf("write pre-commit hook: %w", err)
	}
	return hookPath, nil
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
	return strContains(content, "provmark pre-commit hook")
}

// strContains is a tiny local helper to avoid importing strings for one call.
func strContains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
