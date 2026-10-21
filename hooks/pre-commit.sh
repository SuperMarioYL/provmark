#!/usr/bin/env sh
# provmark pre-commit hook template.
#
# `provmark init` installs a hook based on this script into
# .git/hooks/pre-commit, baking in the absolute path of the provmark binary it
# was run from (falling back to a provmark on PATH if that binary later moves).
#
# The hook is deliberately non-blocking: it prints the unreviewed agent regions
# to stderr and always exits 0, so a commit is never refused — a blocking gate is
# out of scope for provmark. Silence it by reviewing the regions it names:
#
#   provmark review --mark <file>:<start>-<end>
#
# Git runs hooks from the root of the working tree, so --dir . is the repository
# being committed.

PROVMARK="$(command -v provmark 2>/dev/null || true)"
if [ -n "$PROVMARK" ]; then
  "$PROVMARK" review --quiet --dir . >&2 || true
fi
exit 0
