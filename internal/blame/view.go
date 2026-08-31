// Package blame renders a line-level human/agent authorship view beside git
// blame. git blame attributes each line to its committer; provmark blame
// attributes each line to its *author-kind* (human or agent) using the
// provenance sidecar — the field git's data model lacks. This package is the
// view layer: it composes git's per-line metadata with provenance's per-region
// stamps into a single readable diff-like report.
package blame

import (
	"fmt"
	"strings"

	"github.com/SuperMarioYL/provmark/internal/git"
	"github.com/SuperMarioYL/provmark/internal/provenance"
)

const (
	labelHuman = "human"
	labelAgent = "agent"
	// pad widths keep the four columns aligned regardless of SHA/author length.
	padCommit = 7
	padAuthor = 12
	padKind   = 5
	padLine   = 4
)

// Render produces the provmark blame view for one file: one line per source
// line showing, left to right, the line number, the human/agent author-kind,
// the git-blame commit and author, and the source content. Where the sidecar
// has no stamp covering a line, the kind is "-" (unknown) — provmark never
// fabricates attribution it cannot derive from a stamp.
//
// file selects which file's stamps apply: a blame view is per-file, so stamps
// for other files never leak in.
func Render(lines []git.BlameLine, stamps []provenance.Stamp, file string) string {
	if len(lines) == 0 {
		return ""
	}
	fileStamps := filterByFile(stamps, file)
	var b strings.Builder
	b.WriteString(headerRow())
	b.WriteByte('\n')
	for _, ln := range lines {
		kind, session, vendor := LineAuthor(fileStamps, ln.LineNo)
		fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %-*s  %s\n",
			padLine, itoa(ln.LineNo),
			padKind, kind,
			padCommit, ln.Commit,
			padAuthor, truncate(ln.Author, padAuthor),
			ln.Content)
		if kind == labelAgent && (session != "" || vendor != "") {
			b.WriteString(fmt.Sprintf("    %s\n", sessionDetail(session, vendor)))
		}
	}
	return b.String()
}

// LineAuthor returns the author-kind ("human"/"agent") for a given line number
// from the stamp set, plus the session id and vendor for agent lines. A line
// covered by no stamp returns "-" — provmark only attributes lines it has a
// stamp for. Callers should pass stamps already scoped to one file.
func LineAuthor(stamps []provenance.Stamp, lineNo int) (kind, session, vendor string) {
	for _, s := range stamps {
		if lineNo >= s.Region.Start && lineNo <= s.Region.End {
			if s.AuthorKind == provenance.AuthorAgent {
				return labelAgent, s.SessionID, s.AgentVendor
			}
			return labelHuman, "", ""
		}
	}
	return "-", "", ""
}

// filterByFile returns the stamps whose Region.File equals file.
func filterByFile(stamps []provenance.Stamp, file string) []provenance.Stamp {
	var out []provenance.Stamp
	for _, s := range stamps {
		if s.Region.File == file {
			out = append(out, s)
		}
	}
	return out
}

// headerRow is the column legend, padded to match Render's row format.
func headerRow() string {
	return fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %s",
		padLine, "LINE",
		padKind, "KIND",
		padCommit, "COMMIT",
		padAuthor, "GIT AUTHOR",
		"CONTENT")
}

// sessionDetail formats the agent session provenance shown under an agent line.
func sessionDetail(session, vendor string) string {
	if vendor != "" && session != "" {
		return fmt.Sprintf("agent: %s (%s)", session, vendor)
	}
	if vendor != "" {
		return "agent: " + vendor
	}
	return "agent: " + session
}

// truncate clips s to width, appending an ellipsis when it overflows, so the
// git author column stays aligned for long names.
func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}

// itoa is a stdlib-free int->string for the hot per-line path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
