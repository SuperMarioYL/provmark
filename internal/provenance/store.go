package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Load reads the `.provmark.json` sidecar from dir. A missing sidecar is the
// normal pre-stamp state and returns an empty map (no error) so callers like
// the review-gate can treat "not yet stamped" as "nothing to review".
func Load(dir string) (*ProvenanceMap, error) {
	path := filepath.Join(dir, SidecarName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProvenanceMap{}, nil
		}
		return nil, fmt.Errorf("provenance: read sidecar %q: %w", path, err)
	}
	var pm ProvenanceMap
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, fmt.Errorf("provenance: parse sidecar %q: %w", path, err)
	}
	return &pm, nil
}

// Save writes the provenance map to dir/.provmark.json with 2-space indent so
// the committed sidecar diffs cleanly. Save is a plain writer: it does not
// merge prior state, because merging would clobber an in-memory Reviewed flag
// a caller just set (the mark-reviewed path). Re-stamp review preservation is
// the stamp command's job — call MergeReviewState against the prior sidecar
// before Save when regenerating stamps.
func Save(dir string, pm *ProvenanceMap) error {
	if pm == nil {
		return fmt.Errorf("provenance: cannot save nil map")
	}
	path := filepath.Join(dir, SidecarName)
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return fmt.Errorf("provenance: marshal sidecar: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("provenance: write sidecar %q: %w", path, err)
	}
	return nil
}

// MergeReviewState copies Reviewed and StampedAt from prev onto any stamp in
// next whose region and author-kind match, so re-stamping after a partial edit
// preserves prior review decisions for unchanged regions. next is mutated in
// place. This is the re-stamp path only; the mark-reviewed path mutates then
// Saves directly, never through here.
func MergeReviewState(prev, next *ProvenanceMap) {
	if prev == nil || next == nil || len(prev.Stamps) == 0 {
		return
	}
	prevByRegion := make(map[string]Stamp, len(prev.Stamps))
	for _, s := range prev.Stamps {
		prevByRegion[stampKey(s)] = s
	}
	for i, s := range next.Stamps {
		if old, ok := prevByRegion[stampKey(s)]; ok {
			next.Stamps[i].Reviewed = old.Reviewed
			next.Stamps[i].StampedAt = old.StampedAt
		}
	}
}

// stampKey is the stable identity of a stamp across re-stamps: a region plus
// its author-kind. The same physical lines stamped the same way keep their
// review history.
func stampKey(s Stamp) string {
	return s.AuthorKind + "|" + s.Region.File + "|" +
		itoa(s.Region.Start) + ":" + itoa(s.Region.End)
}

// itoa is a stdlib-free int->string for the hot stamp-key path.
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
