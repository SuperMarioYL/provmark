<div align="right"><sub><b>English</b> &nbsp;|&nbsp; <a href="./README.md">简体中文</a></sub></div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/hero-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/hero-light.svg">
  <img src="./assets/hero-light.svg" width="880" alt="provmark — edit-level human-vs-agent authorship stamps">
</picture>

<p align="center"><sub>provmark is the edit-level authorship stamp that separates human-vs-agent lines for agent-coding devs.</sub></p>

**An AI agent edited code alongside you, but `git blame` only shows the committer — provmark stamps every edited region as human or agent at edit granularity, so a review-gate can flag unreviewed agent changes.**

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="license"></a>
  <a href="https://github.com/SuperMarioYL/provmark/releases"><img src="https://img.shields.io/github/v/release/SuperMarioYL/provmark" alt="release"></a>
  <a href="https://github.com/SuperMarioYL/provmark/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/SuperMarioYL/provmark/ci.yml?branch=main&label=ci" alt="ci"></a>
  <img src="https://img.shields.io/badge/Go-1.24-0071E3" alt="go">
</p>

## Contents

- [Architecture](#architecture)
- [Why edit-level](#why-edit-level)
- [Install](#install)
- [Quickstart](#quickstart)
- [Usage](#usage)
- [Demo](#demo)
- [Roadmap](#roadmap)
- [License](#license)

<h2><img src="https://api.iconify.design/tabler:topology-star-3.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Architecture</h2>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/atlas-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/atlas-light.svg">
  <img src="./assets/atlas-light.svg" width="880" alt="architecture: agent session log and git diff feed a correlator that writes a .provmark.json sidecar, read by blame / review / hook">
</picture>

One Go binary, one process. The agent session log is normalised into edit-events by an adapter, aligned hunk-by-hunk against `git diff`, and each region is stamped `human`/`agent` into a committed `.provmark.json` sidecar. `blame`, `review`, and the pre-commit hook all read the same sidecar. No server, no database, no daemon — the durable record carrying provenance is that one JSON file in the commit.

<h2><img src="https://api.iconify.design/tabler:bolt.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Why edit-level</h2>

When agent edits make up most of a commit, `git blame` collapses human and agent changes into a single author — it attributes to the committer and cannot tell which lines inside one commit the agent wrote. That's structural, not "not shipped yet": git's object model is commit→blob→tree with no line-origin field.

provmark fills exactly that missing field: it aligns the agent's session edit-events to `git diff` hunks, stamps each region `human` or `agent`, and writes it into the committed `.provmark.json` sidecar. The starting point is [anthropics/claude-code#66504](https://github.com/anthropics/claude-code/issues/66504) — a 110-upvote dispute over session URLs being injected into commits by default. That argument is for provenance in commits that is intentional and reviewable, not baked in as a side effect. `git blame` gives you the committer; provmark gives you edit-level human/agent attribution.

<h2><img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Install</h2>

```bash
go install github.com/SuperMarioYL/provmark@latest
```

or build from source:

```bash
git clone https://github.com/SuperMarioYL/provmark
cd provmark && go build ./cmd/provmark
```

<h2><img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Quickstart</h2>

Three lines in your repo and you can see the agent attribution `git blame` cannot show:

```bash
cd your-repo
provmark stamp --from agent-session.jsonl --vendor claude-code   # stamp → .provmark.json
provmark blame main.go                                           # line-level human/agent view
```

<details><summary>sample output</summary>

```
$ provmark stamp --from agent-session.jsonl --vendor claude-code
stamped 2 hunk(s) into .provmark.json
  agent: 1   human: 1   (diff base dea69c8)

$ provmark blame app.go
provmark blame app.go (git blame on left, author-kind on right)

LINE  KIND   COMMIT   GIT AUTHOR    CONTENT
1     human  dea69c8  You           package main
4     human  0000000  Not Committ…  	return "human typed this line"
14    agent  dea69c8  You           // 7
    agent: sess-abc (claude-code)
16    agent  dea69c8  You           func agentFn() string {
    agent: sess-abc (claude-code)
17    agent  0000000  Not Committ…  	return "agent generated this line"
    agent: sess-abc (claude-code)
```

</details>

<h2><img src="https://api.iconify.design/tabler:terminal-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Usage</h2>

The core subcommands (full list at `provmark --help`):

```bash
# 1. stamp: align agent edit-events with git diff, write .provmark.json
provmark stamp --from agent-session.jsonl --vendor claude-code

# 2. review-gate: list unreviewed agent regions; --mark marks one reviewed
provmark review
provmark review --mark app.go:15-17

# 3. line-level attribution: human/agent per line beside git blame
provmark blame app.go

# 4. init: install the pre-commit hook (a non-blocking stub in v0.1)
provmark init
```

The **agent edit-event log** is a neutral JSONL contract — one `{"file","start_line","end_line","session_id","vendor"}` per line. `--vendor` selects the adapter: `claude-code` (default), `deepseek-coder`, `qwen-coder`. The format is reconciled against real session logs per `docs/absorption-probe.md`; when a log can't be read it degrades gracefully — every hunk stamps `human`, never a hard failure.

> v0.1 contract: the review-gate only warns, never blocks a commit; stamps are trust-on-first-use with no cryptographic signing.

<h2><img src="https://api.iconify.design/tabler:photo.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Demo</h2>

Ten minutes from `git clone` to seeing line-level human/agent attribution — `git blame` reports the whole commit as "You", while provmark lights up the agent-written lines as `agent`:

![demo](assets/demo.gif)

Reproduce in one command: `bash examples/stamp-review-demo.sh` (needs `go` and `git`).

<h2><img src="https://api.iconify.design/tabler:map-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Roadmap</h2>

- [x] **m1 — stamp + blame + review flagging**: `provmark stamp` aligns agent edit-events with git diff and writes `.provmark.json`; `provmark blame` gives a line-level human/agent view; `provmark review` flags unreviewed agent regions; the absorption probe (`docs/absorption-probe.md`).
- [ ] **m2 — review-gate persistence + hook**: `review --mark` persists `Reviewed` status; the pre-commit hook warns (never blocks) on unreviewed agent regions at commit time.
- [ ] **m3 — before/after demo + bilingual launch**: record the `provmark blame` before/after demo cast; ship the zh README + bilingual sibling; launch on Show HN + 掘金 + V2EX and seed the 17 commenters on claude-code#66504.

The absorption probe is the m1 kill-trigger: if Claude Code or a 国产 coding agent already emits native edit-level human/agent attribution → kill immediately, because the primitive is absorbed into the substrate it rides. The current verdict is in `docs/absorption-probe.md`.

<h2><img src="https://api.iconify.design/tabler:license.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> License</h2>

MIT, see [LICENSE](./LICENSE). Issues and PRs welcome.

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
