# ATD, redroid scaffold, stream_info shipped (feat/orchestrator-core)

Plan: `.agents/plans/2026-09-12-android-atd-redroid-scrcpy.md`. Issue #34
(M2+, moved to WIP on project 20 with progress comment). `task validate`
green after every phase; live tools/list smoke shows 11 tools
(10 existing + stream_info, names unchanged otherwise).

Landed (5 commits):
- `feat(android): ATD image support`: AndroidConfig gains ImageTag/API/
  ABI/RAMSize/HeapSize/AutoProvision (MCPSIM_ANDROID_* env); platforms/
  android/atd.go parses config.ini (`tag.id`, `image.sysdir.1`, handles
  backslash separators); Start appends ATD flags for tagged AVDs;
  sdkmanager/avdmanager auto provisioning behind AutoProvision;
  contract.Device gained additive optional `atd`/`est_ram_mb` fields.
  docs/android-atd.md.
- `feat(redroid): WIP scaffold`: platforms/redroid, all files
  `//go:build linux`; docker run redroid/redroid:12.0.0-latest_64only +
  adb connect; New() returns (nil,nil) when docker missing; bootstrap
  gated on MCPSIM_REDROID_ENABLED (redroid_linux.go/redroid_other.go).
  Linux compile + unit tests verified via throwaway golang:1.25 docker
  container. docs/redroid.md.
- `feat(mcp): stream_info`: guidance-only tool (orchestrator/contract
  untouched). docs/scrcpy.md.

Gotchas:
- `gh project item-add` then `gh project item-list` lags/omits fresh
  items; GraphQL on the project items connection is authoritative for
  finding the item id.
- Iteration field on project 20 is EMPTY (no iterations defined);
  Quarter field is the one with options (Quarter 2 from 2026-08-30).
- Refinement fields to set per policy: Effort + Quarter + Start/Target
  date + Status; classification labels fall back to `enhancement`/`bug`
  (no custom classification labels exist on the repo).
- ghx is just gh (alias); use `gh api graphql` for project field writes.
- piping to `mcp-sim mcp` closes stdin too fast (server EOFs before
  tools/list); wrap the printf in `{ ...; sleep 3; } |` to hold stdin.
- gosec demands 0750/0600 perms in test temp files.
- Android platform registers only when emulator/adb on PATH (this Mac:
  not installed, so smoke is iOS-only). Live ATD smoke still pending.
