// Package version holds build-time version metadata for mcp-sim.
//
// All variables are ldflags-overridable:
//
//	go build -ldflags "-X github.com/espetro/mcp-sim/internal/version.Version=v0.1.1 \
//	                   -X github.com/espetro/mcp-sim/internal/version.Commit=$(git rev-parse HEAD) \
//	                   -X github.com/espetro/mcp-sim/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

// Version is the semantic version of mcp-sim (e.g. "0.1.1").
var Version = "0.1.1"

// Commit is the git commit SHA the binary was built from.
var Commit = "none"

// Date is the build timestamp in RFC3339 format.
var Date = "unknown"

// Human returns a single-line version string for display.
func Human() string {
	return "mcp-sim " + Version + " (" + Commit + ", " + Date + ")"
}
