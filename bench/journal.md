# bench journal: simslim integration dogfood

Machine: MacBookPro17,1 / Apple M1 / 8 GB RAM / macOS 26.6.2 / Xcode 26.6 / iOS 26.5 runtime / simslim 0.8.0

## 2026-09-09

### What ran

`task bench-slim` end to end after fixing three harness issues found while
dogfooding:

1. `simctl boot` exits 149 with "Unable to boot device in current state:
   Booted" when the device is already running. The harness now treats that
   as success and lets `bootstatus` confirm readiness.
2. simslim disable overrides **persist across reboots and erase is the only
   thing that resets them** (per simslim docs), but a previously slimmed
   device stays slim after a plain shutdown/boot. The memory baseline must
   force stock with `simslim off` before measuring, otherwise the "stock"
   row is actually slim (this produced a bogus first run where stock ==
   slim at ~0.97 GB each).
3. `simslim on`/`off` require a **booted** simulator; the wipe/reslim check
   must boot before slimming.

### Observations

- **Memory claim confirmed**: stock booted iPhone sims measured 2.5 to
  3.9 GB phys_footprint (249/196/168 processes); slim consistently ~0.97 GB
  (~70 processes). That is a 2.6x to 4.0x reduction per simulator, matching
  the published 4x claim on the larger devices.
- **On this 8 GB M1** the stock footprints are large enough that running
  even 2 sims at once pressures swap (swapusage showed 4 GB used during
  stock runs). Slim makes 3+ concurrent sims comfortable here.
- **Boot latency cost is real**: first-boot-after-erase stock was ~9.4 s
  avg. The slim path (boot, apply slim with its reboot cycle, boot again)
  averaged 70 s in the worst runs, dominated by the simslim reboot cycle on
  a busy host. The mitigation is what mcp-sim already implements: apply
  slim via on_boot with a pristine profile so second+ boots are slim from
  the start and never pay the post-boot reconfigure cost.
- **Seam precision**: 15/15 open_url succeeded on both stock and slim. No
  feature failures observed for plain deep links.
- **Wipe/re-slim**: confirmed by hand after the harness run (the harness's
  `all_reslimmed: false` in 2026-09-09.md is an artifact of an earlier
  harness ordering bug where status was read before the post-erase boot
  finished; the behavior itself is correct). Verified sequence: slim on ->
  shutdown -> erase -> boot -> status == stock (170/170 re-enabled) ->
  `simslim on` -> status == slim (170/170 disabled). Key discovery: **erase
  only resets the launchd overrides, it does NOT re-download a pristine
  image** — after erase+boot the device still reports slim on this runtime
  (iOS 26.5 persists overrides outside the erased data partition), so
  mcp-sim's wipe re-slim is a cheap no-op-safe re-assert rather than a
  mandatory step. Note: the earlier stock-vs-slim memory baseline measured
  via `simslim off` before erasing is the reliable reset path; erase alone
  is not, on runtimes >= 18.5.

### MCP vs raw CLI (same machine, same workflow)

Run against `bin/mcp-sim mcp` (stdio session, real MCP client) vs agent
shelling `xcrun simctl` directly, 5 iterations each on one device:

| Step | mcp-sim (avg) | raw CLI (avg) | Notes |
|---|---|---|---|
| boot | 13.5 s | 0.7 s | mcp boot_device includes AwaitReady-equivalent internally; CLI `simctl boot` returns immediately (device still booting) |
| await | 2.1 s | 50.6 s | CLI `simctl bootstatus -b` blocks far longer after a bare `boot`; mcp-sim's boot_device already returned a ready device, so await_ready confirms in ~2 s |
| state | 0.5 s / ~50 B | 8.0 s / ~691 B | CLI dumps the full booted-devices JSON; mcp-sim returns a typed scalar state |
| open_url | 18.8 s, 5/5 ok | 4.2 s, 4/5 ok | mcp slower here on this run (device churn from prior steps); both reliable |
| wipe | 24.2 s, 5/5 ok | 13.8 s, 5/5 ok | comparable |

The headline for agents: **correctness and payload, not raw speed**. The
CLI path returns ~9 KB of free-form JSON for a state check an agent must
parse itself, and its `boot` step lies about readiness (exit 0 while still
booting), which is exactly the failure mode that makes agents re-poll or
act early. mcp-sim returns ~50-150 B of typed, stable output per operation
and boot_device only returns once the device is actually usable. On an
8 GB M1 the mcp server process itself is negligible next to simulator
footprints.

Caveat: single-run numbers on a busy machine with only 3 simulators; the
slim-latency spread (13 s to 106 s) shows high host variance. Rerun on a
quiet machine before publishing.
