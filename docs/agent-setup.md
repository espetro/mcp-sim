# Agent setup spec

An imperative task spec for a coding agent to install and configure mcp-sim. A human can paste this whole page (or the README copy) into their agent.

## Context

mcp-sim is an MCP server for iOS Simulator and Android Emulator lifecycle: boot, state, await-ready, wipe, deep-link open. It runs on macOS, Linux, and Windows. iOS support requires macOS with Xcode; Android works on all OSes. Platform adapters are auto-detected: missing tooling means those tools are absent, not a failure. Optional: the `simslim` CLI cuts per-simulator memory ~4x on iOS when configured.

## Steps

1. Install the server:

   ```bash
   go install github.com/espetro/mcp-sim/cmd/mcp-sim@latest
   ```

   (Homebrew `brew install espetro/tap/mcp-sim` is pending; use `go install` for now.)

2. Detect prerequisites and record what is available:

   ```bash
   xcode-select -p        # success: iOS tools will register (macOS only)
   adb version            # success: Android tools will register
   simslim --version      # success: slimming available (optional, iOS)
   ```

   Treat any command that fails as "platform absent", not an error. Proceed with what is present.

3. Verify the binary and see which platforms registered:

   ```bash
   mcp-sim version
   mcp-sim serve --listen 127.0.0.1:9090
   ```

4. Pick a transport for the MCP client in use:
   - **stdio**: client launches the process (Claude Code, most editors). Config: `command: mcp-sim`, `args: ["mcp"]` or `["serve"]`.
   - **HTTP**: client connects to a URL (remote, Tailscale, shared server). Run `mcp-sim serve --listen :9090` and point the client at `http://127.0.0.1:9090` (see [tailscale.md](tailscale.md) for remote access).

5. Write the client config. stdio example:

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

   If the user wants iOS slimming, set `MCPSIM_IOS_SLIM_ENABLED=true` and `MCPSIM_IOS_SLIM_ON_BOOT=true` (or the equivalent YAML config, see [simslim.md](simslim.md)).

6. Restart or reload the MCP client so it picks up the config.

## Verification

Run a `list_devices` round-trip through the client and confirm a JSON array of devices comes back:

- Call the `list_devices` tool. Expect a result like:

  ```json
  {
    "devices": [
      {"platform": "ios", "id": "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX", "name": "iPhone 16", "state": "shutdown"}
    ]
  }
  ```

- Then `boot_device` with that id, `await_ready`, and `get_state`; expect `"state": "running"` (plus an `optimizer` block if slimming is enabled).

## Failure fallbacks

- `mcp-sim: command not found` after `go install`: ensure `$(go env GOPATH)/bin` is on PATH, or use the absolute path in the client config.
- iOS tools missing: `xcode-select --install` or install Xcode from the App Store; then rerun `xcode-select -p`.
- Android tools missing: install Android Studio, ensure `emulator` and `adb` are on PATH (or set `MCPSIM_ANDROID_HOME`).
- Client shows 0 tools: the server did not start or the transport choice is wrong. Check the client's MCP logs, run `mcp-sim serve` in a terminal, and confirm the config command/args match step 4.
- Slimming silently off: `simslim` not on PATH or version < 0.6.0; check `simslim --version`.
