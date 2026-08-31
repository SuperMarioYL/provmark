# Absorption Probe — m1 kill-trigger

> **Status:** run as part of m1. This document is the kill-trigger gate: if the
> dominant agent substrate already emits native edit-level human-vs-agent
> attribution, the provmark primitive is absorbed into the substrate it rides,
> the neutral standard has no job, and the project stops before m2.

## What the probe checks

provmark binds at the **edit-event layer** — it correlates an agent's session
edit-events to `git diff` hunks and stamps each hunk `human` or `agent`. The
absorption question is therefore narrow and falsifiable:

> Does the agent (Claude Code, or a 国产 coding agent such as DeepSeek-Coder /
> Qwen-Coder) already emit a **per-region human-vs-agent authorship field** —
> not a session URL in the commit message, not a tool-call log, but an
> edit-level attribution that a reviewer could read alongside `git blame`?

This is distinct from the adjacent (but different) behaviour that triggered the
seed-pool dispute ([anthropics/claude-code#66504](https://github.com/anthropics/claude-code/issues/66504)):
that issue is about the agent *injecting a session URL into the commit message
by default*. Session-URL injection is provenance as a side-effect, in the wrong
place (the commit message), at the wrong granularity (commit, not edit), and
with no review-gate. It is **not** absorption of provmark's primitive.

## How to run the probe

For each substrate, inspect the agent's session/transcript artefacts for any
field that records, per edited region, whether a human or the agent authored it.

### Claude Code

1. Run a mixed session: type some lines yourself, let the agent edit others.
2. Locate the session transcript (the on-disk record Claude Code keeps).
3. Search it for per-region authorship metadata — a field keyed on file + line
   range with a `human`/`agent` (or equivalent) value.
4. Record the exact field name and shape if found.

### 国产 coding agent (DeepSeek-Coder / Qwen-Coder)

Repeat for one 国产 agent. The CN fragmented-vendor thesis (mvp_plan
`model_targets`) is exactly where a single vendor cannot absorb a neutral
standard — so a 国产 agent is the higher-signal substrate to probe.

## Current finding (m1)

**UNVERIFIED — schema_unverified: true.** The on-disk transcript formats of
Claude Code, DeepSeek-Coder and Qwen-Coder have **not** been independently
confirmed as of this build. provmark's adapters (`internal/agent/adapter.go`)
therefore parse a **documented neutral JSONL contract** (one
`EditEvent{file,start_line,end_line,session_id,vendor}` per line) and **degrade
gracefully**: an unrecognised or missing log yields no events and a warning,
and `provmark stamp` still produces an all-human provenance map rather than
failing.

This means the absorption probe is **open, not closed**: no substrate has been
shown to emit native edit-level attribution, so the neutral-standard thesis
holds and m2 is greenlit. But the probe is not yet *answered with evidence* —
the next step is to reconcile the neutral JSONL contract against each
substrate's real transcript format and update this section with the exact field
names (or the confirmed absence of any).

## Kill criterion

If, on answering the probe, the dominant substrate already ships native
edit-level human-vs-agent attribution (a real per-region field, not a session
URL): **kill immediately** — the primitive is absorbed into the substrate, m2/m3
do not start. This is the m1 gate.

If neither substrate has native attribution (the expected outcome): the
neutral-standard thesis holds, the agent-adapter abstraction is the
absorption-resistant bet, and m2 proceeds.

## Re-check cadence

Re-run this probe whenever a probed agent ships a major release — the
absorption risk is highest at the agent vendor's next attribution release (see
mvp_plan §8.1 and market_forecast §6 risk 1).
