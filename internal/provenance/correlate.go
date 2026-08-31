package provenance

import (
	"time"

	"github.com/SuperMarioYL/provmark/internal/agent"
	"github.com/SuperMarioYL/provmark/internal/git"
)

// Correlate matches each diff hunk to the agent edit-events that overlap it
// and returns one Stamp per hunk. A hunk is stamped "agent" when any edit-event
// on the same file has a line range intersecting the hunk's new-file range;
// the matching event's SessionID and Vendor are carried onto the stamp. A hunk
// with no overlapping event is stamped "human" — the lines git blame would
// attribute to you alone, but that a human actually typed.
func Correlate(hunks []git.Hunk, events []agent.EditEvent) []Stamp {
	if len(hunks) == 0 {
		return nil
	}
	now := time.Now().UTC()
	stamps := make([]Stamp, 0, len(hunks))
	for _, h := range hunks {
		stamp := Stamp{
			Region:     Region{File: h.File, Start: h.StartLine, End: h.EndLine},
			AuthorKind: AuthorHuman,
			Reviewed:   true, // human hunks are reviewed-by-default
			StampedAt:  now,
		}
		if ev, ok := matchingEvent(h, events); ok {
			stamp.AuthorKind = AuthorAgent
			stamp.SessionID = ev.SessionID
			stamp.AgentVendor = ev.Vendor
			stamp.Reviewed = false // agent hunks enter unreviewed
		}
		stamps = append(stamps, stamp)
	}
	return stamps
}

// matchingEvent returns the first agent edit-event whose range overlaps hunk h
// on the same file, if any. Two inclusive ranges [a1,a2] and [b1,b2] overlap
// when a1 <= b2 && b1 <= a2.
func matchingEvent(h git.Hunk, events []agent.EditEvent) (agent.EditEvent, bool) {
	for _, ev := range events {
		if ev.File != h.File {
			continue
		}
		if rangesOverlap(h.StartLine, h.EndLine, ev.StartLine, ev.EndLine) {
			return ev, true
		}
	}
	return agent.EditEvent{}, false
}

// rangesOverlap reports whether inclusive ranges [a1,a2] and [b1,b2] intersect.
func rangesOverlap(a1, a2, b1, b2 int) bool {
	return a1 <= b2 && b1 <= a2
}
