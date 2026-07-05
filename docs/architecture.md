# Architecture

## Adapter model

mcp-sim follows an adapter pattern with two interface layers:

```
MCP client → mcp-sim service → Platform adapters (iOS, Android)
                                → Controller adapters (agent-device)
```

### Platform adapters (`platforms/`)

Implement `contract.Platform` to expose emulator/simulator lifecycle:
- `List`, `Start`, `Stop`, `State`, `AwaitReady`, `Wipe`, `OpenURL`

Built-in: `platforms/ios`, `platforms/android`

### Controller adapters (`controllers/`)

Implement `contract.Controller` to expose verification-layer proxies:
- `Start`, `Stop`, `Status`

Built-in: `controllers/agentdevice`

## Separation of concerns

mcp-sim owns **infrastructure lifecycle only** — power, state, data wipe, deep-link launch.

It does NOT ship UI automation tools (tap, screenshot, view hierarchy). That role belongs to Controllers.

This split is load-bearing: platform adapters must not contain `tap()`, `screenshot()`, or `getTree()` methods. See [CONTRIBUTING.md](../CONTRIBUTING.md).

## State machine

A device transitions through states:

```
Stopped → Booting → Running → Stopped
                ↘ (error) → Error → Stopped
```

## Adapter availability & resilience

Each platform adapter is **lazy** in registration: at server startup, the
adapter constructor probes for its underlying tooling (`xcode-select` for iOS,
`emulator`/`adb` on PATH for Android, etc.). If the tool isn't reachable, the
adapter returns `nil` and the registry simply doesn't include it. The server
boots normally with whatever subset of platforms was actually detected.

This means the same binary can serve a Swift-only developer (no Android SDK),
an Android-only developer (no Xcode), or a full-stack workflow. To disable a
specific platform explicitly:

```bash
MCPSIM_IOS_ENABLED=false
MCPSIM_ANDROID_ENABLED=false
MCPSIM_AGENT_DEVICE_ENABLED=false
```

When an unavailable platform is requested, the call returns a stable MCP
error code (e.g. `unsupported_platform`).

## Extension guide

See [adding-platform.md](adding-platform.md) for implementing a new platform adapter.
