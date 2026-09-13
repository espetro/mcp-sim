# Installing mcp-sim

mcp-sim is distributed as prebuilt binaries on
[GitHub releases](https://github.com/espetro/mcp-sim/releases), as a Homebrew
cask, and via `go install`. Pick whichever channel fits your workflow.

## Homebrew (macOS, recommended)

The primary macOS channel is a Homebrew cask in the `espetro/mcp-sim` tap.
This avoids Gatekeeper entirely: Homebrew strips the quarantine attribute, so
the binary runs without any right click or System Settings dance.

```bash
brew tap espetro/mcp-sim https://github.com/espetro/homebrew-mcp-sim
brew install mcp-sim
```

The cask also registers a launchd service definition. If you want mcp-sim
running in the background you can either run:

```bash
brew services start mcp-sim
```

or use the built in installer described in [Running as a
service](service.md):

```bash
mcp-sim service install --listen :9090
```

## Direct download

Prebuilt archives are attached to every
[release](https://github.com/espetro/mcp-sim/releases): `.zip` for macOS and
Windows, `.tar.gz` for Linux. A `checksums.txt` with SHA256 sums accompanies
each release.

### macOS and Linux

```bash
VERSION=v0.2.0
OS=$(uname -s | tr '[:upper:]' '[:lower:]')   # darwin or linux
ARCH=$(uname -m); [ "$ARCH" = x86_64 ] && ARCH=amd64; [ "$ARCH" = arm64 ] && ARCH=arm64
curl -fsSLO "https://github.com/espetro/mcp-sim/releases/download/${VERSION}/mcp-sim_${VERSION#v}_${OS}_${ARCH}.zip"
curl -fsSLO "https://github.com/espetro/mcp-sim/releases/download/${VERSION}/mcp-sim_${VERSION#v}_checksums.txt"
shasum -a 256 --ignore-missing --check "mcp-sim_${VERSION#v}_checksums.txt"
unzip "mcp-sim_${VERSION#v}_${OS}_${ARCH}.zip"
mv mcp-sim /usr/local/bin/
```

The `shasum` step is your integrity guarantee. On macOS the binaries are
currently unsigned, so Gatekeeper may block a freshly downloaded binary. If
that happens, remove the quarantine attribute:

```bash
xattr -d com.apple.quarantine /usr/local/bin/mcp-sim
```

Prefer the Homebrew channel on macOS to skip this step.

### Windows (PowerShell)

```powershell
$Version = "0.2.0"
$Asset = "mcp-sim_${Version}_windows_amd64.zip"
Invoke-WebRequest "https://github.com/espetro/mcp-sim/releases/download/v${Version}/$Asset" -OutFile $Asset
Invoke-WebRequest "https://github.com/espetro/mcp-sim/releases/download/v${Version}/mcp-sim_${Version}_checksums.txt" -OutFile checksums.txt
Get-FileHash $Asset -Algorithm SHA256
# Compare the output hash against the matching line in checksums.txt
Expand-Archive $Asset -DestinationPath .
Move-Item mcp-sim.exe "$env:LOCALAPPDATA\Microsoft\WindowsApps\"
```

Because the binaries are unsigned, SmartScreen may show a warning the first
time `mcp-sim.exe` runs. The SHA256 check above is the integrity guarantee:
if the hash matches the published checksums, the download is exactly what the
release pipeline produced.

## go install

If you have Go 1.25 or newer:

```bash
go install github.com/espetro/mcp-sim/cmd/mcp-sim@latest
```

Caveat: version information is injected at build time by goreleaser with
`-ldflags`, which `go install` does not do. `mcp-sim version` will report a
development default instead of the real release version. This channel is fine
for trying the tool out or for local development, but prefer Homebrew or the
release archives when you need to know exactly which version is running.

## Verify

```bash
mcp-sim version
```

A normal response names the version, commit, and build date. For release
channel installs these should match the release tag you installed.
