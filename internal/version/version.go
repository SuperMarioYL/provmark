// Package version holds the single provmark version string. go.mod has no
// version field, so this const and the repo-root VERSION file (read by the
// release tooling to tag v<version>) are the two surfaces; they must match.
package version

// Version is the semantic version of this provmark build. Bump it here and in
// the root VERSION file together when cutting a release.
const Version = "0.3.0"
