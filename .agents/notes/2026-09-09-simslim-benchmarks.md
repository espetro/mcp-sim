# 2026-09-09: simslim integration and benchmark findings

Branch `feat/simslim-support` (14 commits ahead of develop, `task validate` green).

## What shipped

- `contract.Optimizer` optional interface (Optimize/Restore/OptimizeStatus/Measure) on Platform; registry type-asserts; MCP surface stays at 10 tools.
- `platforms/ios/slim.go`: simslim CLI wrapper (>= 0.6.0 gate). Profile/except/keep args, `--no-reboot` for iOS < 18.5, wipe re-slim, timeout env scoped to exec'd command only.
- Config: `platforms.ios.slim` + `MCPSIM_IOS_SLIM_*` env vars. Off by default.
- `boot_device` accepts `optimize: true|false` per call; `get_state` returns an `optimizer` block when active.
- Bench harness: `bench/bench-slim` (`task bench-slim`), results in `bench/results/`, journal in `bench/journal.md`.

## Load-bearing findings

1. **simslim state persists across reboots AND across `simctl erase` on iOS >= 18.5** (launchd overrides live outside the erased data partition). `simslim off` is the only reliable reset to stock. Any orchestrator that models "wipe = pristine" will be wrong about slim state on modern runtimes. See `bench/journal.md` (2026-09-09).
2. **Memory claim verified**: 2.6-4.0x phys_footprint reduction (stock 2.4-3.9 GB / 168-249 procs -> slim ~0.97 GB / ~70 procs) on M1 8 GB, iOS 26.5, simslim 0.8.0.
3. **Post-boot slim costs ~1 min** (reconfigure + reboot cycle). Cheap path is slim-at-create (pristine profile before first boot) - second+ boots are slim from the start. Orchestrator API should favor "optimize at provisioning" over "optimize after boot".
4. **MCP vs raw CLI (same machine)**: state check 0.5 s / ~50 B typed vs 8.0 s / ~691 B raw JSON (~14x payload); raw `simctl boot` exits 0 while the device is still booting (false-ready), mcp-sim `boot_device` returns only when usable. This is the token-efficiency evidence base for the orchestrator-core pivot.
5. `simctl boot` exits 149 with "current state: Booted" when already running; `simslim on/off` require a booted device. Harness (and future adapters) must treat both as expected conditions.

## Supersedes

- The plan at `.agents/plans/2026-09-08-simslim-integration.md` is implemented through Phase 2; Phase 3 (release/launch) paused pending the orchestrator-core refactor.
