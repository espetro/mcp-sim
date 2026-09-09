# Product flows (as of 2026-09-09, v0.2.0 + feat/simslim-support)

## Flow 1: Agent boots a device and runs a task (core loop)

```
Agent (MCP client)
   |
   | list_devices                    -> [{id, name, platform, state, version}]
   | boot_device {platform, target}  -> Device (state=booting, resolved only when usable)
   | await_ready {target, timeout}   -> {ready: true}
   | get_state {target}              -> {state, optimizer?{slimmed, persistent, warning, phys_footprint_bytes}}
   | ... agent drives the app (open_url, external tooling) ...
   | wipe_device {target}            -> Device (fresh state; slim re-asserted if on_boot)
   | stop_device {target}            -> Device (stopped)
```

Decision points:
- `boot_device` on a running device -> stable error `already_running`.
- `boot_device` on unknown target -> `unsupported_platform` / not-found error codes.
- `optimize: true|false` overrides `slim.on_boot` for that boot only.

## Flow 2: Remote orchestration (secondary Mac)

```
Main machine                       Secondary Mac
MCP client  --streamable-http-->   mcp-sim serve :9090/mcp
   |                                    |
   |                             iOS Simulator / Android Emulator
   |                                    |
   └-- /healthz liveness check ---------|
```
Setup: see `docs/tailscale.md` (network) and `docs/service.md` (launchd
service install on the secondary machine).

## Flow 3: Memory-slimmed fleet (simslim)

```
config: platforms.ios.slim {enabled, on_boot, profile|except|keep}
   |
   | boot_device -> simslim on <udid> applied right after boot (or --no-reboot on iOS <18.5)
   | get_state   -> optimizer block reports slimmed/persistent/footprint
   | wipe_device -> simslim state re-asserted (erase does NOT reset it on iOS >=18.5)
   |
   └── escape hatch: agent may shell `simslim off <udid>` directly
```

## Flow 4: Controller proxy (agent-device)

```
start_controller {name: agentdevice, port} -> ProxyInfo {url, running}
   ... agent drives UI through proxy ...
stop_controller {name}                     -> ProxyInfo
controller_status {name}                   -> ProxyInfo
```

## What flows do NOT exist yet (orchestrator-core pivot targets)

- No `create/delete/clone device` tools (simulators must pre-exist).
- No engine-specific tool groups (slim verbs live on shell, not MCP).
- No multi-tenant/session isolation of devices between concurrent agents.
