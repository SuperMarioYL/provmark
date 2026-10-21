**English** | [简体中文](README.md)

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/hero-dark.svg">
  <img src="assets/presentation/hero-light.svg" width="1000" alt="Mark Git diff regions using supplied agent edit logs, display provenance beside blame and persist review state.">
</picture>

**Mark Git diff regions using supplied agent edit logs, display provenance beside blame and persist review state.**

`v0.3.0` · `Go 1.24+ / Git` · [MIT](LICENSE)

[Website](https://provmark.lei6393.com) · [Demo record](docs/demo-results.json)

## Why use it

Git records commit authors while people and agents can alternate edits in one working tree. provmark adds provenance based on overlap between logged edits and diff ranges, helping reviewers locate agent-marked regions. It relies on log completeness rather than inferring authorship from code.

## Architecture

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/architecture-dark.svg">
  <img src="assets/presentation/architecture-light.svg" width="1000" alt="Adapters read neutral JSONL edit events, the Git module reads working-tree diffs against HEAD, and the correlator creates one Stamp per hunk. Hunks overlapping an agent event become agent; others default to human. `.provmark.json` stores ranges, sessions and Reviewed state for blame/review.">
</picture>

Adapters read neutral JSONL edit events, the Git module reads working-tree diffs against HEAD, and the correlator creates one Stamp per hunk. Hunks overlapping an agent event become agent; others default to human. `.provmark.json` stores ranges, sessions and Reviewed state for blame/review.

Source entry points: [cmd/provmark/main.go](cmd/provmark/main.go) · [cmd/provmark/hook.go](cmd/provmark/hook.go) · [internal/agent/adapter.go](internal/agent/adapter.go) · [internal/git/diff.go](internal/git/diff.go) · [internal/provenance/correlate.go](internal/provenance/correlate.go) · [internal/provenance/stamp.go](internal/provenance/stamp.go) · [internal/review/gate.go](internal/review/gate.go)

## Install

Requires Go 1.24+ and Git; the self-contained demo also requires Python 3. It creates a synthetic baseline commit in a temporary repository without changing this project’s history.

```bash
git clone https://github.com/SuperMarioYL/provmark.git
cd provmark
go build -o bin/provmark ./cmd/provmark
```

## Quickstart

The example makes two separated edits and supplies a synthetic agent event for the second, then runs stamp, review and mark and inspects the sidecar. No real agent log is used.

```bash
python3 examples/presentation-demo.py
```

Complete inputs and execution steps are included in the commands above and the [demo record](docs/demo-results.json).

## Usage

```bash
./bin/provmark stamp --from events.jsonl --vendor claude-code --dir .
./bin/provmark blame app.go --dir .
./bin/provmark review --dir .
./bin/provmark review --mark app.go:15-17 --dir .
```
Each event is `{"file":"app.go","start_line":15,"end_line":17,"session_id":"session-1","vendor":"claude-code"}`. Ranges use one-based inclusive lines in the current file. Choose mark ranges from actual review output.

## Recorded demo

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/process-dark.svg">
  <img src="assets/presentation/process-light.svg" width="1000" alt="The example makes two separated edits and supplies a synthetic agent event for the second, then runs stamp, review and mark and inspects the sidecar. No real agent log is used.">
</picture>

### Stamp and persist review

Two hunks are marked human and agent; the latter’s unreviewed count changes from 1 to 0.

```text
$ python3 examples/presentation-demo.py
1 unreviewed agent region(s):
  example.py:13-16  claude-code synthetic-agent

review with: provmark review --mark <file>:<start>-<end>
marked 1 agent region(s) reviewed in example.py:13-16
no unreviewed agent regions — clean to commit
{
  "human_hunks": 1,
  "agent_hunks": 1,
  "unreviewed_before": 1,
  "unreviewed_after": 0
}
```

## Capabilities and integration

<picture>
  <source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 640px)" srcset="assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="assets/presentation/integrations-dark.svg">
  <img src="assets/presentation/integrations-light.svg" width="1000" alt="vendor selects claude-code, deepseek-coder or qwen-coder parsers; the names do not prove compatibility with every native session format. init installs a pre-commit hook that lists unreviewed agent regions at commit time and never blocks commits.">
</picture>

vendor selects claude-code, deepseek-coder or qwen-coder parsers; the names do not prove compatibility with every native session format. init installs a pre-commit hook that lists unreviewed agent regions at commit time and never blocks commits.



## Configuration

`--dir` selects the repository, `--from` selects events and `--vendor` selects a parser. stamp covers staged and unstaged changes against HEAD, untracked new files, and staged content in a repo with no commits yet; commit_sha in the sidecar identifies the diff base. Restamping retains prior regions the new diff does not supersede, together with their review state, so committed regions keep their attribution in blame; a region the new diff fully re-stamps is replaced.

## Roadmap and scope

Stamping, blame display, persistent review marks and a commit-time warning about unreviewed agent regions are implemented. Stronger native-log adapters require further validation. Cryptographic provenance proof and blocking commits are outside current capability.

- The unit is a diff hunk (zero context, exactly the changed lines); this is not independent authorship proof for every character.
- Missing or unreadable logs cause hunks to default to human; that label does not establish verified human authorship.

![Terminal recording](assets/demo.gif) · [Recording script](docs/demo.tape)

## License

[MIT](LICENSE)
