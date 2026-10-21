// Command provmark is the edit-level authorship-provenance stamp for
// agent-coding. It separates human-vs-agent lines within a single commit that
// git blame reports under one committer, persists the split as a committed
// `.provmark.json` sidecar, and flags unreviewed agent regions.
//
// v0.1 shipped `stamp`, `blame`, `review` and a stub `init`; v0.2 made
// re-stamps accumulate the sidecar (committed regions keep their attribution)
// and taught stamp to see untracked new files and fresh repos. v0.3 gives
// `init` a real non-blocking hook that warns on unreviewed agent regions at
// commit time, diffs with zero context so a hunk is exactly the changed lines,
// and drops stamps the fresh diff re-stamped.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/provmark/internal/agent"
	"github.com/SuperMarioYL/provmark/internal/blame"
	"github.com/SuperMarioYL/provmark/internal/git"
	"github.com/SuperMarioYL/provmark/internal/provenance"
	"github.com/SuperMarioYL/provmark/internal/review"
	"github.com/SuperMarioYL/provmark/internal/version"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "provmark:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "provmark",
		Short: "edit-level authorship provenance for agent-coding",
		Long: `provmark marks every edited region of a commit as human or agent and
persists the split as a committed .provmark.json sidecar.

git blame attributes a line to its committer; provmark attributes the same
line to its author-kind (human vs agent) — the field git's data model lacks.
The "yes, I want this" moment is seeing the lines an agent wrote light up as
agent inside a commit git blame reports as entirely yours.`,
		Version: version.Version,
	}
	root.AddCommand(newStampCmd(), newBlameCmd(), newReviewCmd(), newInitCmd())
	return root
}

// newStampCmd: provmark stamp --from <log> [--vendor claude-code] [--dir .]
func newStampCmd() *cobra.Command {
	var from, vendor, dir string
	c := &cobra.Command{
		Use:   "stamp",
		Short: "correlate the agent session log with git diff and write .provmark.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStamp(dir, from, vendor, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	c.Flags().StringVar(&from, "from", "", "path to the agent session/edit-event log (JSONL)")
	c.Flags().StringVar(&vendor, "vendor", "claude-code", "agent adapter (claude-code|deepseek-coder|qwen-coder)")
	c.Flags().StringVar(&dir, "dir", ".", "repository root to stamp")
	return c
}

// newBlameCmd: provmark blame <file> [--dir .]
func newBlameCmd() *cobra.Command {
	var dir string
	c := &cobra.Command{
		Use:   "blame <file>",
		Short: "show line-level human/agent authorship beside git blame",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlame(dir, args[0], cmd.OutOrStdout())
		},
	}
	c.Flags().StringVar(&dir, "dir", ".", "repository root to read the sidecar from")
	return c
}

// newReviewCmd: provmark review [--mark <file:start-end>] [--quiet] [--dir .]
func newReviewCmd() *cobra.Command {
	var markTarget, dir string
	var quiet bool
	c := &cobra.Command{
		Use:   "review",
		Short: "list unreviewed agent regions, or mark one reviewed with --mark",
		RunE: func(cmd *cobra.Command, args []string) error {
			if markTarget != "" {
				return runMark(dir, markTarget, cmd.OutOrStdout())
			}
			return runReview(dir, quiet, cmd.OutOrStdout())
		},
	}
	c.Flags().StringVar(&markTarget, "mark", "", "mark <file>:<start>-<end> reviewed (agent regions only)")
	c.Flags().BoolVar(&quiet, "quiet", false, "suppress the all-clean message (used by the pre-commit hook)")
	c.Flags().StringVar(&dir, "dir", ".", "repository root holding the sidecar")
	return c
}

// newInitCmd: provmark init [--dir .] — installs the pre-commit review-gate hook.
func newInitCmd() *cobra.Command {
	var dir string
	c := &cobra.Command{
		Use:   "init",
		Short: "install the pre-commit hook that warns on unreviewed agent regions",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bake the running binary's path into the hook so a commit-time
			// warning works even when provmark is not on PATH; the hook falls
			// back to `command -v provmark` if the binary later moves.
			exe, err := os.Executable()
			if err != nil {
				exe = ""
			}
			return runInit(dir, exe, cmd.OutOrStdout())
		},
	}
	c.Flags().StringVar(&dir, "dir", ".", "repository root to initialise")
	return c
}

func runStamp(dir, from, vendor string, stdout, stderr io.Writer) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := ensureGit(absDir); err != nil {
		return err
	}

	// 1. Parse the agent edit-event stream (degrades gracefully: an unreadable
	//    or unrecognised log yields no events → all hunks stamped human).
	var events []agent.EditEvent
	if from == "" {
		fmt.Fprintln(stderr, "no --from session log given; stamping all hunks as human")
	} else {
		reg := agent.NewRegistry()
		adp, ok := reg.Get(vendor)
		if !ok {
			return fmt.Errorf("unknown agent vendor %q (known: %v)", vendor, reg.Names())
		}
		evs, err := adp.Parse(from)
		if err != nil {
			fmt.Fprintf(stderr, "warning: %v\n", err) // graceful degrade
		}
		events = evs
	}

	// 2. Diff the working tree against HEAD (staged + unstaged). In a repo
	//    with no commits yet, diff against the git empty tree instead — plain
	//    `git diff` there is worktree-vs-index and misses staged files.
	var diffArgs []string
	if sha := git.HeadSHA(absDir); sha != "" {
		diffArgs = append(diffArgs, "HEAD")
	} else {
		diffArgs = append(diffArgs, git.EmptyTreeSHA)
	}
	hunks, err := git.Diff(absDir, diffArgs...)
	if err != nil {
		return fmt.Errorf("read git diff: %w", err)
	}
	// Untracked files never appear in a git diff, so enumerate them as
	// full-file hunks. The sidecar itself is then dropped from both paths —
	// stamping must not record the provenance file as an edit.
	untracked, err := git.UntrackedHunks(absDir)
	if err != nil {
		return fmt.Errorf("list untracked files: %w", err)
	}
	hunks = withoutSidecar(append(hunks, untracked...))
	if len(hunks) == 0 {
		fmt.Fprintln(stderr, "no uncommitted hunks to stamp; nothing to do")
		return nil
	}

	// 3. Correlate, carry prior review decisions for unchanged regions,
	//    retain prior stamps the fresh diff does not supersede, persist.
	pm := provenance.Build(git.HeadSHA(absDir), hunks, events)
	agentCount, humanCount := countByKind(pm)
	if prev, err := provenance.Load(absDir); err == nil {
		provenance.MergeReviewState(prev, pm)
		pm = provenance.Accumulate(prev, pm)
	}
	if err := provenance.Save(absDir, pm); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "stamped %d hunk(s) into %s\n", len(hunks), provenance.SidecarName)
	fmt.Fprintf(stdout, "  agent: %d   human: %d   (%d region(s) recorded, diff base %s)\n",
		agentCount, humanCount, len(pm.Stamps), shaOrWorking(pm.CommitSHA))
	return nil
}

func runBlame(dir, file string, stdout io.Writer) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	pm, err := provenance.Load(absDir)
	if err != nil {
		return err
	}
	lines, err := git.Blame(absDir, file)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "provmark blame %s (git blame on left, author-kind on right)\n\n", file)
	fmt.Fprint(stdout, blame.Render(lines, pm.Stamps, file))
	return nil
}

// runReview lists the unreviewed agent regions. quiet suppresses the all-clean
// message so the pre-commit hook stays silent on a repo with nothing to flag;
// the unreviewed list itself is always printed.
func runReview(dir string, quiet bool, stdout io.Writer) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	pm, err := provenance.Load(absDir)
	if err != nil {
		return err
	}
	unreviewed := review.Unreviewed(pm)
	if len(unreviewed) == 0 {
		if !quiet {
			fmt.Fprintln(stdout, "no unreviewed agent regions — clean to commit")
		}
		return nil
	}
	fmt.Fprintf(stdout, "%d unreviewed agent region(s):\n", len(unreviewed))
	for _, s := range unreviewed {
		fmt.Fprintf(stdout, "  %s:%d-%d  %s\n", s.Region.File, s.Region.Start, s.Region.End, agentDetail(s))
	}
	fmt.Fprintln(stdout, "\nreview with: provmark review --mark <file>:<start>-<end>")
	return nil
}

func runMark(dir, target string, stdout io.Writer) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	file, start, end, err := review.ParseRange(target)
	if err != nil {
		return err
	}
	n, err := review.Mark(absDir, file, start, end)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "marked %d agent region(s) reviewed in %s\n", n, target)
	return nil
}

// runInit installs the pre-commit hook, baking provmarkPath into it so the
// commit-time review warning works without provmark on PATH.
func runInit(dir, provmarkPath string, stdout io.Writer) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := ensureGit(absDir); err != nil {
		return err
	}
	hookPath, err := installPreCommitHook(absDir, provmarkPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "installed pre-commit hook at %s\n", hookPath)
	fmt.Fprintln(stdout, "note: the hook warns about unreviewed agent regions and never blocks a commit")
	return nil
}

// withoutSidecar drops hunks for the provenance sidecar itself. The sidecar is
// untracked on the first stamp but becomes an ordinary tracked file afterwards,
// so a re-stamp would otherwise record `.provmark.json` as an edited region.
func withoutSidecar(hunks []git.Hunk) []git.Hunk {
	out := make([]git.Hunk, 0, len(hunks))
	for _, h := range hunks {
		if h.File != provenance.SidecarName {
			out = append(out, h)
		}
	}
	return out
}

// ensureGit verifies dir is inside a git repo and returns a clear error otherwise.
func ensureGit(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		// not at the root — check whether we're inside a repo at all
		if !isGitRepo(dir) {
			return fmt.Errorf("%q is not a git repository (run `git init` first)", dir)
		}
	}
	return nil
}

// isGitRepo reports whether git considers dir a working tree.
func isGitRepo(dir string) bool {
	return runGitOK(dir, "rev-parse", "--is-inside-work-tree")
}

// runGitOK runs a git command and reports whether it succeeded (output discarded).
func runGitOK(dir string, args ...string) bool {
	full := append([]string{"-C", dir}, args...)
	cmd := execCommand("git", full...)
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run() == nil
}

// countByKind splits the stamps into agent/human totals for the stamp summary.
func countByKind(pm *provenance.ProvenanceMap) (agentN, humanN int) {
	for _, s := range pm.Stamps {
		switch s.AuthorKind {
		case provenance.AuthorAgent:
			agentN++
		case provenance.AuthorHuman:
			humanN++
		}
	}
	return
}

// shaOrWorking renders the commit-sha field for a summary line: "WORKING" when
// the sidecar was stamped against an empty repo (no HEAD yet).
func shaOrWorking(sha string) string {
	if sha == "" {
		return "WORKING (no HEAD)"
	}
	return sha
}

// agentDetail formats the session/vendor of an unreviewed agent stamp.
func agentDetail(s provenance.Stamp) string {
	if s.AgentVendor != "" && s.SessionID != "" {
		return s.AgentVendor + " " + s.SessionID
	}
	if s.AgentVendor != "" {
		return s.AgentVendor
	}
	return s.SessionID
}
