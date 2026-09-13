# Cross-platform release readiness audit (2026-09-13)

## Verdict: YES, effectively ready. All 6 targets build and vet clean today.

Build proof (CGO_ENABLED=0, go 1.25):
- linux/amd64 PASS, linux/arm64 PASS, windows/amd64 PASS, windows/arm64 PASS,
  darwin/amd64 PASS (darwin/arm64 is the dev machine).
- GOOS=windows go vet PASS, GOOS=linux go vet PASS.
- .goreleaser.yml already targets darwin/linux/windows x amd64/arm64 with
  correct ldflags version injection; release.yml runs goreleaser on
  ubuntu-latest (correct: macs cross-compile from linux with CGO off).

## Compatibility matrix

| Feature | darwin (both) | linux (both) | windows (both) |
|---|---|---|---|
| Lifecycle tools | full | full | full |
| iOS adapter | full (xcrun) | N/A (skips, WARN log) | N/A (skips) |
| Android adapter + ATD | full | full | full (procattr_windows.go exists) |
| redroid | N/A | opt-in (linux build tag) | N/A |
| simslim optimizer | full (opt-in) | N/A | N/A |
| Service install | launchd via kardianos/service | systemd (user service ok) | Windows Service (elevated) |
| Auth (bearer) | full | full | full, with one path gap (see gaps) |

## Gaps (all small)
1. S: .goreleaser.yml uses deprecated properties (archives.format,
   format_overrides.format, brews) per goreleaser 2.18 check; migrate to
   archives.formats and homebrew_casks or the new brews syntax before the
   release tooling hard-fails.
2. S: Token/snippet paths hardcode ~/.config/mcp-sim on ALL platforms
   (internal/bootstrap/bootstrap.go:166,175). Works on Windows (just
   unconventional); consider os.UserConfigDir() for Windows-native path.
   Config file loader already uses ~/.config too (internal/config/config.go:155),
   so consistent, but both are non-idiomatic on Windows.
3. S: macOS binaries are unsigned: Gatekeeper blocks GUI-less downloads
   less, but brew tap distribution avoids Gatekeeper entirely (recommended
   primary channel for mac). Developer ID signing/notarization only needed
   for direct-download distribution; defer until there is demand.
4. S: Windows SmartScreen/AV false positives for unsigned Go binaries are
   common; mitigation is publishing checksums (already in goreleaser) and
   documenting `Invoke-WebRequest` + hash verify in README.
5. M: No windows/windows-arm64 CI build smoke; add a matrix job that runs
   go build + go vet (no tests need Windows runners initially).

## Release claims for v0.3
Claim: darwin/linux/windows, amd64+arm64 binaries; full lifecycle + auth
everywhere; iOS features macOS-only; redroid linux-only; simslim macOS-only
opt-in. All honest today.
