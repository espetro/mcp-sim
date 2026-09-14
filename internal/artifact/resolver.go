// Package artifact resolves artifact references (absolute paths, paths under
// registered roots, or artifact:// URIs) to absolute filesystem paths.
//
// Roots come from the MCPSIM_ARTIFACT_ROOTS environment variable,
// colon-separated like PATH. Two forms per entry:
//
//	"/abs/path"          -> anonymous root, matched by relative refs in order
//	"name=/abs/path"     -> named root, addressable as artifact://name/rel
//
// This is the only new abstraction install_app introduces: where the artifact
// comes from (xcodebuild, gradle, eas, a CI cache) stays the caller's problem.
package artifact

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Root is one entry of MCPSIM_ARTIFACT_ROOTS.
type Root struct {
	Name string // alias for artifact:// refs; empty for anonymous roots
	Path string // absolute filesystem path
}

// ErrNotFound reports that a ref did not resolve under any registered root.
var ErrNotFound = errors.New("artifact: ref did not resolve under any MCPSIM_ARTIFACT_ROOTS entry")

// LoadRoots parses the MCPSIM_ARTIFACT_ROOTS value (colon-separated). Empty
// entries are skipped; "name=path" entries become named roots.
func LoadRoots(env string) []Root {
	var roots []Root
	for _, entry := range strings.Split(env, ":") {
		if entry == "" {
			continue
		}
		// "name=path" is a named root; a path containing "=" (e.g.
		// "/opt/=x=/y") is not, because the alias would start with "/".
		if i := strings.Index(entry, "="); i > 0 && !strings.HasPrefix(entry[:i], "/") {
			roots = append(roots, Root{Name: entry[:i], Path: entry[i+1:]})
			continue
		}
		roots = append(roots, Root{Path: entry})
	}
	return roots
}

// Resolve turns a ref into an absolute filesystem path. Accepted forms:
//
//   - absolute path: returned as-is (cleaned)
//   - "artifact://<name>/<rel>": joined under the named root
//   - relative path (including "artifact://<rel>"): joined under the first
//     root where the candidate exists on disk
func Resolve(ref string, roots []Root) (string, error) {
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref), nil
	}

	rel := ref
	if strings.HasPrefix(ref, "artifact://") {
		rest := strings.TrimPrefix(ref, "artifact://")
		// artifact://<name>/<rel> resolves against the named root.
		if i := strings.Index(rest, "/"); i > 0 {
			for _, r := range roots {
				if r.Name == rest[:i] {
					return filepath.Join(r.Path, rest[i+1:]), nil
				}
			}
		}
		rel = rest
	}

	for _, r := range roots {
		candidate := filepath.Join(r.Path, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
}
