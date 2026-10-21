package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoFile reads a repo-root-relative file. Tests run with cwd set to this
// package's directory (internal/version), so the repo root is two levels up.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestVersionLockstepAcrossSurfaces is the mechanical guard for the version
// string, which is duplicated across six surfaces that are hand-bumped at
// release time. go.mod carries no version field, so nothing else fails when they
// drift; this test does.
func TestVersionLockstepAcrossSurfaces(t *testing.T) {
	v := Version
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(v) {
		t.Fatalf("version.Version = %q is not MAJOR.MINOR.PATCH", v)
	}

	if got := strings.TrimSpace(repoFile(t, "VERSION")); got != v {
		t.Errorf("VERSION file = %q, version.Version = %q", got, v)
	}
	if src := repoFile(t, "internal/version/version.go"); !strings.Contains(src, `const Version = "`+v+`"`) {
		t.Errorf("version.go does not declare `const Version = %q`", v)
	}

	var site struct {
		Meta struct {
			ContentVersion string `json:"content_version"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(repoFile(t, "web/site.json")), &site); err != nil {
		t.Fatalf("parse web/site.json: %v", err)
	}
	if site.Meta.ContentVersion != v {
		t.Errorf("web/site.json meta.content_version = %q, want %q", site.Meta.ContentVersion, v)
	}
	if raw := repoFile(t, "web/site.json"); !strings.Contains(raw, `"`+v+` · Go`) {
		t.Errorf("web/site.json footer tag does not carry version %q", v)
	}

	for _, rel := range []string{"README.md", "README.en.md"} {
		if !strings.Contains(repoFile(t, rel), "`v"+v+"`") {
			t.Errorf("%s does not carry the `v%s` version line", rel, v)
		}
	}
	if !strings.Contains(repoFile(t, "CHANGELOG.md"), "## ["+v+"]") {
		t.Errorf("CHANGELOG.md has no `## [%s]` heading", v)
	}
}
