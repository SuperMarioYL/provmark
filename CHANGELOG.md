# Changelog

All notable changes to provmark are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-09-16

### Fixed

- Re-stamps now **accumulate** the sidecar instead of replacing it. In 0.1.0
  every `provmark stamp` overwrote the whole `.provmark.json`, silently
  discarding regions recorded by earlier stamps — committed agent regions
  vanished and `provmark blame` regressed to `-` (unknown) on lines it used
  to attribute. Fresh stamps supersede prior stamps with the same
  region+kind key; prior stamps not re-stamped are retained with their
  review history. The stamp summary now reports both freshly stamped hunks
  and total regions recorded.
- `provmark stamp` now sees **untracked new files** — the typical agent
  output. `git diff` never lists untracked files, so a brand-new
  agent-written file stamped as "no uncommitted hunks to stamp" and
  `provmark review` reported "clean to commit". Untracked files are now
  enumerated via `git ls-files --others --exclude-standard` and stamped as
  full-file hunks (ignored files excluded).
- In a repo with no commits yet, stamp now diffs against the git empty tree
  so **staged files are stampable**. Plain `git diff` there is
  worktree-vs-index and showed nothing for a staged file.
- The sidecar itself is excluded from untracked enumeration, so stamping
  never records `.provmark.json` as an edit.
- Demo fixes: mark the actual agent region (`app.go:15-17`; the old range
  marked 0 regions and left the demo unclean) and keep session logs outside
  the fixture repo.

### Added

- Cmd-level end-to-end tests (`cmd/provmark/main_test.go`) locking the
  stamp → review → mark → blame happy path plus both fix regressions
  (committed-region retention across re-stamp; untracked and fresh-repo
  file stamping). `cmd/provmark` previously shipped zero tests.

## [0.1.0] - 2026-08-31

### Added

- Initial release: `provmark stamp`, `provmark review`, `provmark blame`
  and a stub `provmark init`. Correlates neutral JSONL agent edit events
  with working-tree diff hunks, labels each region human or agent (with
  session and vendor), persists the split as a committed `.provmark.json`
  sidecar, and flags unreviewed agent regions in review and blame output.
- Vendor parsers: `claude-code`, `deepseek-coder`, `qwen-coder`.
- Bilingual product site (GitHub Pages) and a self-contained demo.
