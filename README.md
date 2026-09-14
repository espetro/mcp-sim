# mcp-sim

**MCP server for mobile emulator lifecycle**

`mcp-sim` turns iOS Simulators and Android Emulators into first-class MCP tools. Boot on demand, verify state, tear down when done: emulators live exactly as long as the work that needs them. Runs on macOS, Linux, and Windows. The lifecycle core is also a plain Go library (`pkg/orchestrator`) you can embed in your own programs.

## Install (run this)

```bash
go install github.com/espetro/mcp-sim/cmd/mcp-sim@latest
```

Homebrew: `brew install espetro/tap/mcp-sim` (tap pending; use `go install` until then). Prebuilt binaries: [releases](https://github.com/espetro/mcp-sim/releases).

If `go` is missing: install Go 1.25+ first, then rerun the command above.

## Prerequisites

Each platform is auto-detected at startup. A missing tool means that platform's MCP tools are absent; the server still starts.

| Tool | Required for | OS | Detection command | Install if missing |
|------|---|---|---|---|
| Xcode + iOS Simulators | iOS tools | macOS only | `xcode-select -p` | `xcode-select --install` or App Store |
| Android SDK (`emulator`, `adb`) | Android tools | all | `adb version` | [Android Studio](https://developer.android.com/studio) |
| `simslim` >= 0.6.0 (optional) | iOS memory slimming | macOS | `simslim --version` | [simslim](docs/simslim.md) |
| `agent-device` (optional) | verification controller | all | `agent-device version` | [agent-device](https://github.com/espetro/agent-device) |

## MCP tools

| Tool | Description |
|------|-------------|
| `list_devices` | List all available emulators/simulators |
| `boot_device` | Boot a device by platform and target (optional `optimize` flag when [simslim](docs/simslim.md) is enabled) |
| `stop_device` | Stop a running device |
| `wipe_device` | Wipe device user data (re-applies slimming when `on_boot` is set) |
| `get_state` | Get device state (includes an `optimizer` block when slimming is active) |
| `await_ready` | Wait for device to finish booting |
| `open_url` | Open a deep link on a device |
| `install_app` | Install an app artifact ([`artifact_ref` forms](docs/install.md); requires `MCPSIM_ARTIFACT_ROOTS` for relative/artifact:// refs) |
| `launch_app` | Launch an installed app by bundle id, returns pid |
| `start_controller` | Start a controller proxy daemon |

## What mcp-sim is NOT

mcp-sim stays deliberately narrow: device lifecycle plus install and launch.

- **No build tool.** `xcodebuild`, `gradle`, `eas build` live elsewhere (XcodeBuildMCP, DroidPilot, your CI step).
- **No transfer tool.** No scp/rsync/curl helpers; the runner already has those.
- **No artifact orchestration.** No version resolution, no EAS/Bitrise API integration, no remote (HTTP/s3) artifact fetching. Callers produce a local path and pass it in.
- **No uninstall (yet).** Deferred until a concrete CI need shows up.

See [docs/install.md](docs/install.md) for the install/launch surface.

## Config keys and env vars

Precedence: CLI flags > env vars > YAML (`~/.config/mcp-sim/config.yaml`, or `$MCPSIM_CONFIG`).

| Env var | YAML key | Meaning |
|---|---|---|
| `MCPSIM_LISTEN` | `server.listen` | HTTP listen address (default `:9090`) |
| `MCPSIM_LOG_LEVEL` | `server.log_level` | log level |
| `MCPSIM_LOG_FORMAT` | `server.log_format` | log format |
| `MCPSIM_CONFIG` | (n/a) | path to config file |
| `MCPSIM_AUTH_TOKEN` | `server.auth.token` | bearer token for `/mcp` (HTTP); see [docs/auth.md](docs/auth.md) |
| `MCPSIM_INSECURE_NO_AUTH` | `server.auth.enabled` (inverted) | disable bearer auth (gated on non-loopback listeners) |
| `MCPSIM_ARTIFACT_ROOTS` | (n/a) | colon-separated artifact roots for `install_app` (`name=path` for `artifact://` refs); see [docs/install.md](docs/install.md) |
| `MCPSIM_TRUSTED_NETWORK` | (n/a) | set `true` to acknowledge a trusted network and satisfy the insecure-no-auth gate |
| `MCPSIM_IOS_ENABLED` | `platforms.ios.enabled` | force iOS adapter on/off |
| `MCPSIM_DEVELOPER_DIR` | `platforms.ios.developer_dir` | xcode-select developer dir |
| `MCPSIM_IOS_SLIM_ENABLED` | `platforms.ios.slim.enabled` | enable simslim integration |
| `MCPSIM_IOS_SLIM_ON_BOOT` | `platforms.ios.slim.on_boot` | slim during `boot_device` |
| `MCPSIM_IOS_SLIM_PROFILE` | `platforms.ios.slim.profile` | path to slim profile JSON |
| `MCPSIM_IOS_SLIM_EXCEPT` | `platforms.ios.slim.except` | category IDs to keep |
| `MCPSIM_IOS_SLIM_KEEP` | `platforms.ios.slim.keep` | launchd labels to keep |
| `MCPSIM_IOS_SLIM_BOOT_TIMEOUT` | `platforms.ios.slim.boot_timeout` | passthrough to simslim |
| `MCPSIM_IOS_SLIM_SPAWN_TIMEOUT` | `platforms.ios.slim.spawn_timeout` | passthrough to simslim |
| `MCPSIM_ANDROID_ENABLED` | `platforms.android.enabled` | force Android adapter on/off |
| `MCPSIM_ANDROID_HOME` | `platforms.android.android_home` | ANDROID_HOME override |
| `MCPSIM_JAVA_HOME` | `platforms.android.java_home` | JAVA_HOME override |
| `MCPSIM_ANDROID_EMULATOR_BIN` | `platforms.android.emulator_bin` | emulator binary path |
| `MCPSIM_AGENT_DEVICE_ENABLED` | `controllers.agentdevice.enabled` | force controller on/off |
| `MCPSIM_AGENT_DEVICE_PORT` | `controllers.agentdevice.proxy_port` | controller proxy port |

## Client config (stdio)

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

For HTTP/remote setups see [docs/tailscale.md](docs/tailscale.md); for a native OS service see [docs/service.md](docs/service.md).

## Verify your setup

Run these and check the outputs:

```bash
mcp-sim version
# expect: mcp-sim <version> (commit, date)

mcp-sim serve --listen 127.0.0.1:9090
# expect: server starts, prints registered platforms
```

Then call the `list_devices` MCP tool through your client. Expect JSON like:

```json
{
  "devices": [
    {"platform": "ios", "id": "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX", "name": "iPhone 16", "state": "shutdown"}
  ]
}
```

Then `boot_device` + `await_ready` + `get_state`; expect `"state": "running"`.

## Agent setup prompt

Paste this into your coding agent. The full spec with failure fallbacks is [docs/agent-setup.md](docs/agent-setup.md).

```text
Install and configure mcp-sim, an MCP server for iOS Simulator and Android
Emulator lifecycle.

1. Install: go install github.com/espetro/mcp-sim/cmd/mcp-sim@latest
2. Detect prerequisites: run `xcode-select -p`, `adb version`, and
   `simslim --version`. A failing command means that platform is absent,
   not an error; proceed with what is available.
3. Run `mcp-sim version` to confirm the install.
4. Pick the transport for my MCP client: stdio if the client launches
   processes, HTTP (mcp-sim serve --listen 127.0.0.1:9090) if it connects
   to a URL.
5. Write my client config accordingly (stdio: command "mcp-sim", args
   ["serve", "--listen", "127.0.0.1:9090"]).
6. Verify with a list_devices round-trip: expect a JSON devices array.
   Then boot_device + await_ready + get_state and confirm
   "state": "running".

If anything fails, follow the fallbacks in docs/agent-setup.md:
https://espetro.github.io/mcp-sim/agent-setup
```

## Benchmarks

Measured on a MacBook Pro M1 (8 GB), Xcode 26.6, iOS 26.5 runtime, simslim 0.8.0. Method: phys_footprint via `simslim measure --json`, 3 devices, 5 iterations. Full data in `bench/results/`.

| Metric | Stock | Slim | Delta |
|---|---|---|---|
| phys_footprint per simulator | 2.4 to 3.9 GB | ~0.97 GB | 2.6 to 4.0x reduction |
| Processes per simulator | 168 to 249 | ~70 | fewer daemons |
| open_url deep-link success | 15/15 | 15/15 | no seam breakage |

Boot cost: applying slim post-boot adds a reconfigure+reboot cycle (~1 min worst case on this 8 GB host). With `slim.on_boot` (the default flow in mcp-sim), second and later boots are slim from the start and do not pay that cost.

## Embedding (Go library)

The lifecycle core is a public Go package:

```go
o, err := orchestrator.New(
    orchestrator.WithPlatform(ios.New(ios.Defaults())),
    orchestrator.WithLogger(slog.Default()),
)
devices, _ := o.List(ctx)
_ = o.Boot(ctx, devices[0].ID)
state, _ := o.State(ctx, devices[0].ID)
```

See [docs/architecture.md](docs/architecture.md).

## Docs

Hosted: **https://espetro.github.io/mcp-sim/**

- [Agent setup](docs/agent-setup.md): copyable setup spec for coding agents
- [Authentication](docs/auth.md): bearer token auth for the HTTP surface
- [Architecture](docs/architecture.md): layers and embedding
- [simslim integration](docs/simslim.md): optional iOS simulator memory slimming (~4x)
- [Tailscale setup](docs/tailscale.md): running over Tailscale
- [Running as a service](docs/service.md): launchd/systemd/Windows Service
- [Adding a platform](docs/adding-platform.md): implementing the Platform interface

## License

Apache 2.0
