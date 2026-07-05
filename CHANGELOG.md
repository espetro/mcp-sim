# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing yet.

<!--
## [X.Y.Z] - YYYY-MM-DD

### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security
-->

## [Unreleased]

Nothing yet.

<!--
## [X.Y.Z] - YYYY-MM-DD

### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security
-->

## [0.1.1] - 2026-07-05

### Fixed
- **Android emulator killed by request ctx** — `exec.CommandContext(reqCtx)` in `platforms/android.Start()` was tying the emulator process lifetime to the MCP request. Cancelling the request (which the SDK does on response) sent SIGKILL to the emulator shortly after boot_device returned. Now spawned with `exec.CommandContext(context.Background(), ...)`.
- **Android emulator stdio inherited from server** — if the server is backgrounded with a closed stdout/stderr pipe, the emulator gets SIGPIPE on its first log line. stdio now redirected to `io.Discard`.
- **`agent-device` controller restoration** — initial v0.1.0 controller was tested against an older agent-device v0.14 that didn't ship the `proxy` subcommand. v0.1.1 restores proxy-spawning behavior using `agent-device proxy --port N --host 127.0.0.1 [--daemon-auth-token ...]`, matching agent-device v0.18+ (the current version). PID tracking + SIGTERM-on-stop pattern, same as the Android emulator.
- **`open_url` errors had no context** — `cmd.Run()` returned just `exit status 1`. Now uses `CombinedOutput()` and includes stderr so callers see e.g. "Activity not started, unable to resolve Intent".
- **`stop_device` didn't reliably terminate Android emulator** — relied on `adb emu kill` which can race. Now tracks spawned PIDs and SIGTERMs the process group directly.
- **`await_ready` default timeout too short** — bumped from 60s to 180s (iOS Simulator boot on slower Macs takes ~90s).
- **`boot_device` default timeout too short** — bumped from 60s to 120s in lifecycle.go.
- **AGENTS.md release content** — removed the `## Current release` section (went stale at every release); release docs now live in `docs/releases/` and AGENTS.md references them.

### Added
- `docs/releases/v0.1.0.md` — release notes for the v0.1.0 tag.
- `.agents/plans/2026-07-05-v0.1.0-integration-test.md` — integration test report.
- `.agents/plans/2026-07-05-v0.1.1-release.md` — release plan for v0.1.1.

## [0.1.0] - 2026-07-05

### Added
- MCP server with streamable HTTP transport (`/mcp`) and stdio transport (`mcp` subcommand)
- iOS Simulator adapter via `xcrun simctl`
- Android Emulator adapter via `emulator` + `adb`
- agent-device controller adapter
- 10 MCP tools: `list_devices`, `boot_device`, `stop_device`, `get_state`, `await_ready`, `wipe_device`, `open_url`, `start_controller`, `stop_controller`, `controller_status`
- Lazy platform detection: iOS/Android register only if their tooling is reachable
- Configuration via YAML (`~/.config/mcp-sim/config.yaml`) or env vars (`MCPSIM_*`)
- Structured logging via `log/slog`
- Graceful shutdown on SIGINT/SIGTERM
- `/healthz` HTTP endpoint
- `mcp-sim --help`, `serve --help`, `mcp --help`, and `version` subcommands
- Cross-compiled releases via GoReleaser (darwin/linux/windows × amd64/arm64)
- Homebrew tap `espetro/homebrew-mcp-sim` (when published)

[Unreleased]: https://github.com/espetro/mcp-sim/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/espetro/mcp-sim/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/espetro/mcp-sim/releases/tag/v0.1.0
