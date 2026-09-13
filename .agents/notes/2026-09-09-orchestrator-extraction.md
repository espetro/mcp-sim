# Orchestrator core extraction shipped (feat/orchestrator-core)

Plan: `.agents/plans/2026-09-09-orchestrator-core.md`. Issue #31 (project 20);
follow-up stubs #32 toolsets/meta-tool, #33 RemotePlatform, #34 Android ATD.
Decision record: `.agents/docs/ARCHITECTURE.md`.

Landed (6 commits, `task validate` green, live smoke ok):
- `pkg/contract`: `CapabilitySet` uint32 bitmask + `Capabilities()` required on
  `Platform`; `As[T]` escape hatch; `CapabilitiesFor` helper.
- `pkg/orchestrator`: concrete `Orchestrator`, fallible `Option`s, duplicate
  registration errors, idempotent Boot/Stop, eventually-consistent `List`
  (error only when ALL platforms fail), `ShutdownAll(ctx)`. Imports only
  pkg/contract + stdlib. Unexported `profileReconciler` iface is the
  reconciliation hook.
- Wipe reconciliation: `Platform.ReconcileProfile` on iOS (skip when slimmed,
  no-op when not enabled/on_boot); runs after Wipe and Boot. MCP surface
  unchanged (10 tools verified via tools/list smoke).
- Behavior deltas (intended): boot on running device now succeeds with
  reconciliation; stop on stopped is no-op; wipe re-applies slim baseline.
- `internal/core` deleted; single `bootstrap.BuildOrchestrator` wires
  serve/mcp/service modes.
- `pkg/orchestrator/conformance.RunSuite` wired into ios (passes vs real
  simulators, ~32s) and android (skips, no SDK). `go-apidiff` CI on
  pkg/contract + pkg/orchestrator. bench-slim orchestrator smoke behind
  `MCPSIM_BENCH_TARGET`.

Gotchas:
- naming a fake's field `state` masks the promoted `State` method from an
  embedded struct (silent zero value) — use `devState`.
- gosec G602 flags `want[i]` in `for i := range got` even after length check;
  write explicit bounds.
- MCP streamable HTTP requires initialize + session header before tools/list.
- bench-slim binary lands in repo root when built manually; rm before commit.
- CHANGELOG had a duplicated `## [Unreleased]` header (pre-existing).
- version lives in `internal/version/version.go` line 11 (now 0.3.0-dev).
