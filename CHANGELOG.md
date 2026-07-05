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

[Unreleased]: https://github.com/espetro/mcp-sim/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/espetro/mcp-sim/releases/tag/v0.1.0
