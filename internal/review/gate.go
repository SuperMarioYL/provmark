// Package review is the review-gate: a read/write over the provenance
// sidecar's Reviewed flag. `provmark review` lists the agent hunks a human has
// not yet reviewed (AuthorKind == "agent" && !Reviewed); `provmark review
// --mark <file>:<start>-<end>` flips Reviewed=true and persists. v0.1 only
// ever warns — it never blocks a commit (blocking is explicitly out of scope).
package review

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SuperMarioYL/provmark/internal/provenance"
)

// Unreviewed returns the agent stamps still awaiting human review. These are
// the regions the review-gate flags — the whole reason provmark separates
// agent from human edits: to make the unreviewed-agent subset visible.
func Unreviewed(pm *provenance.ProvenanceMap) []provenance.Stamp {
	if pm == nil {
		return nil
	}
	var out []provenance.Stamp
	for _, s := range pm.Stamps {
		if s.AuthorKind == provenance.AuthorAgent && !s.Reviewed {
			out = append(out, s)
		}
	}
	return out
}

// Mark flips Reviewed=true for every agent stamp whose region intersects the
// given file/range and persists the sidecar in dir. It returns the count of
// stamps it marked. Marking a range that touches no agent stamp is a no-op
// (and a helpful zero count), so the review-gate stays non-blocking even on a
// miss: nothing fails, the user is just told nothing matched.
func Mark(dir, file string, start, end int) (int, error) {
	pm, err := provenance.Load(dir)
	if err != nil {
		return 0, fmt.Errorf("review: load sidecar: %w", err)
	}
	marked := 0
	for i := range pm.Stamps {
		s := &pm.Stamps[i]
		if s.AuthorKind != provenance.AuthorAgent {
			continue
		}
		if s.Region.File != file {
			continue
		}
		if rangesOverlap(s.Region.Start, s.Region.End, start, end) {
			s.Reviewed = true
			marked++
		}
	}
	if marked == 0 {
		return 0, nil
	}
	if err := provenance.Save(dir, pm); err != nil {
		return 0, fmt.Errorf("review: persist sidecar: %w", err)
	}
	return marked, nil
}

// ParseRange parses a "<file>:<start>-<end>" or "<file>:<start>" review-mark
// target (1-based, inclusive end). A single number means a one-line range.
func ParseRange(s string) (file string, start, end int, err error) {
	idx := strings.LastIndex(s, ":")
	if idx < 0 || idx == len(s)-1 {
		return "", 0, 0, fmt.Errorf("review: range must be <file>:<start>-<end>, got %q", s)
	}
	file = s[:idx]
	rest := s[idx+1:]
	parts := strings.SplitN(rest, "-", 2)
	start, err = strconv.Atoi(parts[0])
	if err != nil || start < 1 {
		return "", 0, 0, fmt.Errorf("review: invalid start line %q in %q", parts[0], s)
	}
	end = start
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil || end < start {
			return "", 0, 0, fmt.Errorf("review: invalid end line %q in %q", parts[1], s)
		}
	}
	return file, start, end, nil
}

// rangesOverlap reports whether inclusive ranges [a1,a2] and [b1,b2] intersect.
func rangesOverlap(a1, a2, b1, b2 int) bool {
	return a1 <= b2 && b1 <= a2
}
