# mcp-sim

**MCP server for mobile emulator lifecycle**

`mcp-sim` turns remote iOS Simulators and Android Emulators into first-class MCP tools. Boot on demand, verify via your favorite agent, tear down on completion — emulators live exactly as long as the work that needs them.

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
- [launchd](docs/launchd.md) — macOS service management
- [Adding a platform](docs/adding-platform.md) — implementing the Platform interface

## License

Apache 2.0
