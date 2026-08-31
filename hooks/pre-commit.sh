#!/usr/bin/env sh
# provmark pre-commit hook — v0.1 stub
#
# This is the committed hook TEMPLATE. `provmark init` copies it into
# .git/hooks/pre-commit. v0.1 ships a non-blocking no-op: the review-gate
# warning that surfaces unreviewed agent regions (without blocking the commit)
# is m2 scope and lands here when m2 ships:
#
#   provmark review --dir "$PWD" || true   # warns, never blocks (v0.1 contract)
#
# m2 implementation will run the review-gate and print the unreviewed agent
# regions to stderr, then exit 0 regardless — provmark never blocks a commit.
exit 0
