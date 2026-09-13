# Low-level surfaces: CLI / YAML / env (as of 2026-09-09)

## Commands

```
mcp-sim serve [--listen :9090]     # HTTP/SSE MCP server (default :9090)
mcp-sim mcp                        # stdio MCP server
mcp-sim service install|uninstall|start|stop|restart|status|run [--user]
mcp-sim version
```

## Config resolution order

CLI flag > env var > YAML file > default. YAML path: `$MCPSIM_CONFIG` or
`~/.config/mcp-sim/config.yaml`.

## YAML keys and env vars (source: internal/config/config.go)

| YAML | Env | Default | Notes |
|---|---|---|---|
| server.listen | MCPSIM_LISTEN | :9090 | |
| server.log_level | MCPSIM_LOG_LEVEL | info | |
| server.log_format | MCPSIM_LOG_FORMAT | text | |
| platforms.ios.enabled | MCPSIM_IOS_ENABLED | true | adapter self-skips without Xcode |
| platforms.ios.developer_dir | MCPSIM_DEVELOPER_DIR | (xcode-select -p) | |
| platforms.ios.slim.enabled | MCPSIM_IOS_SLIM_ENABLED | false | probes simslim >= 0.6.0 on PATH |
| platforms.ios.slim.on_boot | MCPSIM_IOS_SLIM_ON_BOOT | false | slim during boot_device |
| platforms.ios.slim.profile | MCPSIM_IOS_SLIM_PROFILE | "" | simslim profile JSON path |
| platforms.ios.slim.except | MCPSIM_IOS_SLIM_EXCEPT | [] | category IDs, CSV env / YAML list |
| platforms.ios.slim.keep | MCPSIM_IOS_SLIM_KEEP | [] | launchd labels, CSV env / YAML list |
| platforms.ios.slim.boot_timeout | MCPSIM_IOS_SLIM_BOOT_TIMEOUT | (simslim default 10m) | SIMSLIM_BOOT_TIMEOUT passthrough |
| platforms.ios.slim.spawn_timeout | MCPSIM_IOS_SLIM_SPAWN_TIMEOUT | (simslim 2m) | SIMSLIM_SPAWN_TIMEOUT passthrough |
| platforms.android.enabled | MCPSIM_ANDROID_ENABLED | true | self-skips without emulator/adb |
| platforms.android.android_home | MCPSIM_ANDROID_HOME | | |
| platforms.android.java_home | MCPSIM_JAVA_HOME | | |
| platforms.android.emulator_bin | MCPSIM_ANDROID_EMULATOR_BIN | | |
| controllers.agentdevice.enabled | MCPSIM_AGENT_DEVICE_ENABLED | true | |
| controllers.agentdevice.proxy_port | MCPSIM_AGENT_DEVICE_PORT | 9000 | |

## External binaries probed at startup

| Binary | Gate | Missing means |
|---|---|---|
| xcode-select / xcrun | `xcode-select -p` succeeds | iOS tools absent, server runs |
| simslim | `exec.LookPath` + `simslim version` >= 0.6.0 | optimizer nil, everything else fine |
| emulator / adb | PATH | Android tools absent |
| agent-device | PATH | controller absent |

## Registration seams (for the orchestrator-core refactor)

- `internal/bootstrap.BuildHTTPServer` (serve mode) and
  `cmd/mcp-sim/main.go: mcpImpl` (stdio mode) both duplicate platform
  registration - Phase 1 of the pivot unifies these into one
  `bootstrap.BuildRegistry(cfg)`.
- Public contract: `pkg/contract` (Platform, Controller, Optimizer,
  StartOpts). Orchestrator logic currently internal (`internal/core`).
