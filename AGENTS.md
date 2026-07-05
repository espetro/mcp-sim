# mcp-sim — Agent Guide

AI agent context for the mcp-sim Go project (mobile emulator lifecycle MCP server).

## Project Overview

mcp-sim is an MCP server for managing iOS Simulator and Android Emulator lifecycle,
runnable on macOS, Linux, or Windows. Built-in adapters for iOS (xcrun simctl, macOS-only —
requires Xcode), Android (emulator + adb, cross-platform), and agent-device controller
(cross-platform).

## Backlog

We use a [GitHub project](https://github.com/users/espetro/projects/20) as project backlog.

Issues #1–#21 cover M1 milestone tasks. M2 hardening tracked separately.

## Current release

v0.1.1 — patch release with bug fixes. Branch `main` holds released commits. Use `develop` for
new work. Versions live in `internal/version/version.go` and are injected at
build time via `-ldflags "-X .../internal/version.Version=..."`.

## Key Documentation

| Document | Purpose |
|----------|---------|
| `docs/architecture.md` | Adapter model + separation of concerns |
| `docs/tailscale.md` | Network deployment over Tailscale |
| `docs/service.md` | Running mcp-sim as a native OS service (launchd/systemd/Windows Service) |
| `docs/adding-platform.md` | Implementing new Platform adapters |
| `CONTRIBUTING.md` | PR process + load-bearing separation rule |
| `pkg/contract/platform.go` | Platform interface |
| `pkg/contract/controller.go` | Controller interface |

## Agent Workflow

1. **Run validate**: `task validate` (typecheck + lint + test)
2. **Build**: `task build` produces `bin/mcp-sim`
3. **Run locally**: `task run -- --listen :9090`
4. **Read per-package docs** before modifying adapters

### Commit style

- Atomic commits: each commit is a single self-contained logical change
- Conventional commit format (feat:, fix:, chore:, docs:, ci:)
- Pass CI locally before committing

### Branches

- `main`: protected, release commits only
- `develop`: integration branch, feature branches merge here
- `release/vX.Y.Z`: release branches
- `hotfix/vX.Y.Z`: emergency fixes from main

## Stack Summary

- **Language**: Go 1.25+
- **MCP**: `github.com/modelcontextprotocol/go-sdk` v1.6+
- **Config**: `gopkg.in/yaml.v3` + stdlib `os.Getenv`
- **Logging**: stdlib `log/slog`
- **HTTP**: stdlib `net/http` (1.22+ `ServeMux`)

## Package Overview

```
cmd/mcp-sim/            Entry point: serve | mcp | version
pkg/contract/           Platform + Controller interfaces (public)
pkg/mcp/                MCP server wiring (public)
internal/config/        CLI > env > file > default resolution
internal/log/           slog wrapper
internal/http/          HTTP server with graceful shutdown
internal/core/          Registry, lifecycle, tool handlers
platforms/ios/          iOS Simulator adapter (xcrun simctl)
platforms/android/      Android Emulator adapter (emulator + adb)
controllers/agentdevice/ agent-device proxy adapter
```