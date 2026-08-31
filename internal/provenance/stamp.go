// Package provenance owns the edit-level authorship provenance map — the
// core primitive provmark introduces. git attributes at commit granularity
// (the committer); provenance attributes at edit granularity *within* a
// commit, persisted as a committed `.provmark.json` sidecar so the durable git
// record carries it.
package provenance

import (
	"time"

	"github.com/SuperMarioYL/provmark/internal/agent"
	"github.com/SuperMarioYL/provmark/internal/git"
)

// Region is a line range in a file's working-tree version: Start and End are
// 1-based, End inclusive. It is the shared coordinate space across git.Hunk
// and agent.EditEvent, so correlation is plain line overlap.
type Region struct {
	File  string `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// Stamp is one hunk's authorship record: who/what authored the region, which
// agent session it came from, and whether a human has reviewed it. AuthorKind
// is "human" or "agent" — the field git's data model lacks.
type Stamp struct {
	Region      Region    `json:"region"`
	AuthorKind  string    `json:"author_kind"` // "human" | "agent"
	SessionID   string    `json:"session_id,omitempty"`
	AgentVendor string    `json:"agent_vendor,omitempty"`
	Reviewed    bool      `json:"reviewed"`
	StampedAt   time.Time `json:"stamped_at"`
}

// ProvenanceMap is the committed sidecar: every changed hunk in the working
// tree, stamped with its author-kind. CommitSHA is the diff base (HEAD at
// stamp time), recorded so a reader knows what the stamps were diffed against.
type ProvenanceMap struct {
	CommitSHA string  `json:"commit_sha"`
	Stamps    []Stamp `json:"stamps"`
}

const (
	// SidecarName is the committed JSON file that carries the provenance map.
	SidecarName = ".provmark.json"
	AuthorHuman = "human"
	AuthorAgent = "agent"
)

// Build correlates the diff hunks against the agent edit-events and produces a
// provenance map: each hunk becomes a Stamp, marked "agent" when an agent
// edit-event overlaps it, otherwise "human". This is the edit-level split
// git blame cannot make — the whole point of provmark.
func Build(commitSHA string, hunks []git.Hunk, events []agent.EditEvent) *ProvenanceMap {
	stamps := Correlate(hunks, events)
	return &ProvenanceMap{CommitSHA: commitSHA, Stamps: stamps}
}
