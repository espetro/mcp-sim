# Verifier (device interaction)

The verifier tool family lets an agent interact with a booted device: tap
elements by ref, type text, swipe, and read the screen. It is implemented on
top of callstack's [agent-device](https://github.com/callstack/agent-device),
spawned by mcp-sim as a child MCP server over stdio (`agent-device mcp`).

## Tools

| Tool | Purpose |
|------|---------|
| `verifier_snapshot` | Capture the accessibility hierarchy: elements with stable refs |
| `verifier_tap` / `verifier_double_tap` | Tap an element by snapshot ref |
| `verifier_long_press` | Long-press an element (duration_ms, default 500) |
| `verifier_swipe` | Swipe up/down/left/right |
| `verifier_type` | Type text into an element by ref |
| `verifier_press_button` | Press a hardware/system button (home, back, ...) |
| `verifier_open_url` | Open a URL or deep link |
| `verifier_screenshot` | Capture the screen as a base64 PNG |

Typical loop: `verifier_snapshot` to see the screen, then
`verifier_tap`/`verifier_type` with the refs it returned.

## Enabling

The verifier is opt-in-but-automatic: with no configuration, it turns on when
an `agent-device` binary is found on `PATH` and stays off (silently) when it
is not. The `verifier_*` tools are always advertised; without a backend they
return a structured `verifier_unavailable` error.

Precedence (CLI-free: env > config file > binary detection):

| Setting | Env | YAML | Meaning |
|---------|-----|------|---------|
| enabled | `MCPSIM_AGENT_DEVICE_VERIFIER` | `controllers.agentdevice.verifier` | Explicit on/off; overrides detection |
| binary path | `MCPSIM_AGENT_DEVICE_BIN` | `controllers.agentdevice.bin_path` | Binary location (default: `$PATH` lookup) |

Example `~/.config/mcp-sim/config.yaml`:

```yaml
controllers:
  agentdevice:
    enabled: true
    verifier: true
    # bin_path: /opt/homebrew/bin/agent-device
```

Install agent-device (macOS, via Homebrew):

```sh
brew install callstack/tap/agent-device
```

agent-device itself is cross-platform; on Linux/Windows point `bin_path` at
your build.

## Architecture notes

- The `contract.Verifier` interface (`pkg/contract/verifier.go`) is the seam:
  a future Maestro or Appium backend implements the same ten methods and
  registers through `orchestrator.WithVerifier`. Only one backend is active
  at a time.
- The adapter (`controllers/agentdevice/verifier.go`) is a thin dispatcher:
  each method maps onto one agent-device MCP tool call, so tap/snapshot logic
  stays in agent-device rather than being re-implemented here.
- Graceful degradation mirrors platforms: a missing backend never prevents
  the server from starting.
