# mcp-sim

> **MCP server for iOS Simulator and Android Emulator lifecycle on a remote macOS host.**

Boot on demand, verify via your agent, tear down on completion - emulators live exactly as long as the work that needs them.

## Features

- **Lifecycle-only by design** - boot, stop, wipe, deep-link. Tap, screenshot, and view hierarchy are deliberately out of scope; those belong to verification controllers like [agent-device](https://github.com/callstack/agent-device).
- **Lazy adapter registration** - iOS, Android, and agent-device are auto-detected at startup. The same binary serves Swift-only, Android-only, or full-stack users.
- **MCP-over-HTTP + stdio** - long-lived service for remote hosts, stdio transport for per-agent spawn, both via the official [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk).

## Quick start

```bash
brew install espetro/mcp-sim/mcp-sim
mcp-sim serve --listen :9090
```

Wire it into your MCP client (Claude Code, Cursor, etc.):

```json
{
  "mcpServers": {
    "mcp-sim": {
      "command": "mcp-sim",
      "args": ["serve", "--listen", "127.0.0.1:9090"]
    }
  }
}
```

<details>
<summary>Other install methods</summary>

### `go install`

```bash
go install github.com/espetro/mcp-sim/cmd/mcp-sim@latest
```

### Pre-built binaries

Download for darwin/linux/windows × amd64/arm64 from
[github.com/espetro/mcp-sim/releases](https://github.com/espetro/mcp-sim/releases).

### Build from source

```bash
git clone https://github.com/espetro/mcp-sim.git
cd mcp-sim
task build   # produces ./bin/mcp-sim
```

</details>

## Requirements

| What | Why | Install |
|------|-----|---------|
| macOS host | emulators run on Mac | — |
| Xcode (latest stable) | iOS Simulator | App Store or `xcode-select --install` |
| Android SDK with `emulator` + `adb` on PATH | Android Emulator | [Android Studio](https://developer.android.com/studio) or `brew install --cask android-commandlinetools` |
| [agent-device](https://github.com/callstack/agent-device) v0.18+ | verification controller | `npm install -g agent-device@latest` |

If any tool is missing at startup, that platform's MCP tools are simply absent — the server still starts and the rest works. Force-disable with `MCPSIM_IOS_ENABLED=false`, `MCPSIM_ANDROID_ENABLED=false`, or `MCPSIM_AGENT_DEVICE_ENABLED=false`.

## What sets it apart

- **Infrastructure vs verification split** - mcp-sim owns emulator lifecycle (power, state, data wipe, deep-link). Verification (taps, screenshots, accessibility) lives in controllers. The split is enforced in `CONTRIBUTING.md`; PRs that conflate them are closed.
- **Tailscale-friendly by default** - bind to your Mac's Tailscale IP and an agent on a Linux VPS can drive emulators over the tailnet. See [docs/tailscale.md](docs/tailscale.md).
- **Single static binary** - `CGO_ENABLED=0`, no runtime dependencies, ~12 MB. Ships via Homebrew tap, `go install`, and GitHub Releases.

## Skills

For AI agents setting up or driving mcp-sim:

- **Agent setup recipe**: see `.agents/plans/` (gitignored — local-only references for each milestone, e.g. `2026-07-05-v0.1.0-integration-test.md`)
- **MCP-server-builder conventions**: see the [`mcp-builder`](https://github.com/modelcontextprotocol/go-sdk) reference

## Docs

- [Architecture](docs/architecture.md) — adapter model, separation of concerns, resilience
- [Tailscale setup](docs/tailscale.md) — remote-host deployment
- [launchd](docs/launchd.md) — macOS service management
- [Adding a platform](docs/adding-platform.md) — implementing the `Platform` interface
- [Releases](docs/releases/) — per-version release notes
- [CHANGELOG](CHANGELOG.md) — full change log

## License

Apache 2.0