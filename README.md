# mcp-sim

**MCP server for mobile emulator lifecycle**

`mcp-sim` turns remote iOS Simulators and Android Emulators into first-class MCP tools. Boot on demand, verify via your favorite agent, tear down on completion — emulators live exactly as long as the work that needs them.

## Prerequisites

mcp-sim is resilient: each platform adapter is auto-detected at startup, and a missing tool just means that platform's tools are skipped. You can run the server with **only iOS**, **only Android**, or **both**.

The server itself runs on macOS, Linux, and Windows. iOS support requires macOS + Xcode (Simulator is Apple-only tooling); Android and the agent-device controller work on all three OSes.

| Tool | Required for | OS | Install |
|------|---|---|---|
| Xcode + iOS Simulators | iOS tools | macOS only | `xcode-select --install` (or App Store → Xcode) |
| Android SDK + `emulator` + `adb` | Android tools | macOS, Linux, Windows | Install [Android Studio](https://developer.android.com/studio) or `brew install --cask android-commandlinetools` |
| `agent-device` | verification controller | macOS, Linux, Windows | `brew install agent-device` (or see [agent-device docs](https://github.com/espetro/agent-device)) |

Each tool is checked via PATH probing at server startup. If `xcode-select -p` succeeds the iOS adapter registers; if `emulator` or `adb` is on PATH, the Android adapter registers; if `agent-device` resolves, the controller registers. Otherwise the relevant MCP tools are simply absent — the server still starts.

To force a platform off:

```bash
MCPSIM_IOS_ENABLED=false mcp-sim serve
MCPSIM_ANDROID_ENABLED=false mcp-sim serve
MCPSIM_AGENT_DEVICE_ENABLED=false mcp-sim serve
```

## Install

### Homebrew (macOS/Linux)

```bash
brew install espetro/mcp-sim/mcp-sim
```

### go install

```bash
go install github.com/espetro/mcp-sim/cmd/mcp-sim@latest
```

### GitHub Releases

Download pre-built binaries from [github.com/espetro/mcp-sim/releases](https://github.com/espetro/mcp-sim/releases).

## Quick start

Start the server:

```bash
mcp-sim serve --listen :9090
```

Show all commands:

```bash
mcp-sim --help
mcp-sim serve --help
```

Show version:

```bash
mcp-sim version
# mcp-sim 0.1.1 (commit, date)
```

Configure your MCP client (Claude Code, Cursor, etc.):

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

For Tailscale-based remote access, see [docs/tailscale.md](docs/tailscale.md).

## Tools

| Tool | Description |
|------|-------------|
| `list_devices` | List all available emulators/simulators |
| `boot_device` | Boot a device by platform and target |
| `stop_device` | Stop a running device |
| `get_state` | Get device state |
| `await_ready` | Wait for device to finish booting |
| `wipe_device` | Wipe device user data |
| `open_url` | Open a deep link on a device |
| `start_controller` | Start a controller proxy daemon |
| `stop_controller` | Stop a controller proxy daemon |
| `controller_status` | Check controller proxy status |

## Docs

- [Architecture](docs/architecture.md) — adapter model and separation of concerns
- [Tailscale setup](docs/tailscale.md) — running over Tailscale
- [Running as a service](docs/service.md) — install as a native OS service (launchd/systemd/Windows Service)
- [Adding a platform](docs/adding-platform.md) — implementing the Platform interface

## License

Apache 2.0
