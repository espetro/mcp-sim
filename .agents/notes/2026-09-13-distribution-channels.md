# Distribution channels research

Date: 2026-09-13
Status: research only, no implementation
Context: releases ship via goreleaser with zip/tar.gz archives plus
checksums.txt. macOS binaries are unsigned. Homebrew cask tap
(espetro/homebrew-mcp-sim) is live as the primary macOS channel.

## mise

mise installs non stdlib tools through several backends:

1. aqua registry: mise uses the aqua registry as a first party backend. Adding
   mcp-sim means a PR to aquaproj/aqua-registry adding a package manifest that
   points at our GitHub release assets (name, version source, files, and a
   checksum source which goreleaser's checksums.txt already satisfies). Other
   Go CLI projects almost universally use this route because the registry has
   tens of thousands of packages wired to GitHub releases.
2. ubi backend: mise can install GitHub release binaries directly via the
   generic ubi backend with no registry PR, e.g. `mise use
   ubi:espetro/mcp-sim`. This can work today if release assets follow the
   conventional `name_os_arch` naming, which ours do.
3. asdf plugin: a small custom repo with a bin/list-all and bin/install
   script. More maintenance, mostly superseded by backends.

Effort: option 2 is zero effort and testable now. Option 1 is a single
upstream PR, roughly an hour including the aqua registry contribution
guidelines check. Recommendation: document ubi usage first, submit the aqua
registry PR later.

## npm binary wrapper

The esbuild and turbo pattern: a thin platform independent npm package that
uses optionalDependencies for each platform specific subpackage
(@espetro/mcp-sim-darwin-arm64 etc) each containing the prebuilt binary. npm
installs only the subpackage matching the current platform.

goreleaser note: the `npms` publish pipe exists, but it is a goreleaser Pro
feature. Also its approach differs from esbuild's: it generates a package
whose postinstall script downloads and extracts the release archive, rather
than shipping binaries as optionalDependencies. That means a network fetch at
install time and no platform subpackages.

For an MCP server this channel has outsized value: agents start MCP servers
via `npx -y mcp-sim serve` inside JSON client config, which is the most agent
friendly launch path possible and sidesteps all install steps. This is how
most MCP servers in the wild are distributed. Effort with goreleaser Pro:
small (config only). Effort hand rolled: medium, roughly a day to build
publisher scripts for 6 platform subpackages, launchers, and CI plumbing.

## scoop and winget (Windows)

scoop: goreleaser (OSS) has a `scoops` pipe that commits a manifest to a
scoop bucket repo. Effort is trivial, same shape as the existing
homebrew_casks config, needs a bucket repo such as espetro/scoop-bucket.
Worth doing when Windows users show up.

winget: no goreleaser pipe. Requires a manifest PR per release to
microsoft/winget-pkgs (YAML manifests, can be generated with winget-create
from the release installer or archive URL). Automated via GitHub Actions
running winget-create on release. Higher friction and review latency
upstream. Recommendation: scoop now, winget only on demand.

## MCP registry (high value)

goreleaser's `mcp` publish pipe (landed in v2.13, refined through v2.18)
generates a server.json conforming to the MCP schema and publishes it to the
official Model Context Protocol registry. It wraps mcp-publisher so no local
tooling is needed.

Shape for us:

```yaml
mcp:
  name: "io.github.espetro/mcp-sim"
  title: "mcp-sim"
  repository:
    url: "https://github.com/espetro/mcp-sim"
    source: "github"
  auth:
    type: "github-oidc"
  packages:
    - registry_type: npm
      identifier: "@espetro/mcp-sim"
      transport:
        type: stdio
```

Requirements and gotchas:

- The name io.github.espetro/mcp-sim ties ownership to the GitHub org and
  requires the publish token to come from that org (github-oidc in CI is the
  supported path; DNS verification also possible).
- registry_type values are oci, npm, pypi, nuget, mcpb. For a stdio Go
  binary the natural entries are npm (if we publish the wrapper) or a direct
  download style reference. If we publish via npm, package.json must carry
  the extra field `mcpName: io.github.espetro/mcp-sim` or the MCP publish
  fails.
- disable: auto skips publishing on prerelease tags.

This is likely the highest value channel for this project: being listed in
the MCP registry makes mcp-sim discoverable by MCP clients and agent
config flows. It requires the npm wrapper (or an OCI image) to exist as the
referenced package, so the npm work becomes a prerequisite. Effort after
npm: small, mostly config and one CI secret for OIDC.

## Suggested order

1. MCP registry via npm wrapper (highest value, agents are the users).
2. mise via ubi (zero effort documentation win).
3. scoop pipe (trivial once a bucket repo exists).
4. aqua registry PR, winget, as demand appears.
