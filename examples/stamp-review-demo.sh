#!/usr/bin/env bash
# provmark end-to-end demo — see edit-level human/agent attribution that
# git blame cannot show.
#
# This script builds provmark, creates a throwaway fixture repo, simulates a
# mixed human/agent editing session (two non-contiguous edits → two diff
# hunks), and runs `stamp`, `blame`, and `review` so you can watch git blame's
# committer-only view sit beside provmark's per-line human/agent split.
#
# Run:  bash examples/stamp-review-demo.sh
# Needs: go + git on PATH. Leaves a fixture repo under /tmp/provmark-demo.
set -euo pipefail

# Resolve the repo root from this script's location (works from any cwd).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Build provmark into a temp bin (prefers a `provmark` on PATH if present).
BIN="$(command -v provmark || true)"
if [ -z "$BIN" ]; then
  BIN="/tmp/provmark-demo-bin"
  echo "building provmark → $BIN"
  (cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/provmark)
fi

DEMO="/tmp/provmark-demo"
rm -rf "$DEMO"
mkdir -p "$DEMO"
cd "$DEMO"
git init -q
git config user.email "you@example.com"
git config user.name "You"

# 1. Baseline: a small Go file. The two functions are separated by a ~10-line
#    unchanged block so the later human and agent edits (lines 4 and 16) sit
#    far enough apart that `git diff` emits TWO hunks, not one — which is the
#    only way provmark can stamp one hunk human and the other agent.
cat > app.go <<'EOF'
package main

func humanFn() string {
	return "baseline human"
}

// --- separator: keeps the human and agent edits in separate diff hunks ---
// (provmark diffs with -U0, so each change is its own hunk; this block just
//  keeps the fixture readable)
// 2
// 3
// 4
// 5
// 6
// 7

func agentFn() string {
	return "placeholder"
}

func main() {
	println(humanFn())
	println(agentFn())
}
EOF
git add app.go
git commit -qm "baseline"

# 2. Mixed session: a HUMAN edits humanFn (line 4), an AGENT edits agentFn
#    (line 17). The two changes are ~12 unchanged lines apart, so `git diff -U0`
#    produces two hunks — hunk 1 (human, line 4) and hunk 2 (agent, line 17).
#    git blame collapses both into one committer ("You"); provmark splits them
#    by author-kind — the field git's data model lacks.
cat > app.go <<'EOF'
package main

func humanFn() string {
	return "human typed this line"
}

// --- separator: keeps the human and agent edits in separate diff hunks ---
// (provmark diffs with -U0, so each change is its own hunk; this block just
//  keeps the fixture readable)
// 2
// 3
// 4
// 5
// 6
// 7

func agentFn() string {
	return "agent generated this line"
}

func main() {
	println(humanFn())
	println(agentFn())
}
EOF

# 3. The agent's session edit-event log (neutral JSONL contract provmark parses).
#    Its range (lines 15-17) overlaps only the changed line 17, so with the
#    zero-context diff exactly line 17 is marked agent; humanFn's edit has no
#    event, so provmark stamps that hunk human. (schema_unverified: real agent
#    logs differ; this fixture is what an adapter reconciles against.) The log
#    lives OUTSIDE the fixture repo so untracked-file stamping sees exactly the
#    two app.go hunks the demo narrates.
SESSION_LOG="$DEMO-session.jsonl"
cat > "$SESSION_LOG" <<EOF
{"file":"app.go","start_line":15,"end_line":17,"session_id":"sess-abc","vendor":"claude-code"}
EOF

echo
echo "=========================================================="
echo " 1. provmark stamp  — correlate agent events with git diff"
echo "=========================================================="
"$BIN" stamp --from "$SESSION_LOG" --dir .

echo
echo "=========================================================="
echo " 2. git blame app.go  — can only show the committer (You)"
echo "=========================================================="
git blame app.go | sed 's/^/  /'

echo
echo "=========================================================="
echo " 3. provmark blame app.go  — line-level human/agent split"
echo "=========================================================="
"$BIN" blame app.go --dir .

echo
echo "=========================================================="
echo " 4. provmark review  — flag unreviewed agent region(s)"
echo "=========================================================="
"$BIN" review --dir .

echo
echo "=========================================================="
echo " 5. provmark review --mark app.go:15-17  — mark it reviewed"
echo "=========================================================="
"$BIN" review --mark app.go:15-17 --dir .

echo
echo "=========================================================="
echo " 6. provmark review  — now clean"
echo "=========================================================="
"$BIN" review --dir .

echo
echo "done. fixture repo left at: $DEMO"
echo "inspect the sidecar:  cat $DEMO/.provmark.json"
