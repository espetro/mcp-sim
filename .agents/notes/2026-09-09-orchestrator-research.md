# 2026-09-09: orchestrator-core pivot research (tool surfaces, Go API, engine landscape)

Research for the pivot: MCP (API) <-> Orchestrator Core <-> Simulator Adapter.
Full findings merged into `.agents/docs/screens/../notes/` and the draft
assessment in `.agents/drafts/2026-09-09-orchestrator-assessment.md`.

## 1. Extensible MCP tools without context bloat

- **Consensus mechanism: static startup registration + named capability groups.**
  Playwright-MCP `--caps` (core ~22 tools always on, pdf/vision/devtools opt-in);
  Google MCP Toolbox "toolsets" (named groups, selectable per agent, even
  per-toolset HTTP endpoints). Verdict for mcp-sim: 5 core lifecycle tools
  always registered; each engine adapter ships a *toolset* the operator
  enables in config. Toggleable meta-tool per user requirement.
- **Do NOT rely on `tools/list_changed`**: broken in Codex (never refetches,
  openai/codex#33266), partially broken in Claude Code (anthropics/claude-code#66084),
  MCP spec 2026-07-28 moved notifications to opt-in subscriptions. Emit it as
  courtesy only.
- **Meta-tool / tool-search**: Anthropic Tool Search Tool (GA): 72K tokens of
  schemas -> ~500 token search tool; accuracy 49%->74% on 50+ tool servers.
  Rule of thumb: skip below ~10 tools. Naive nested-JSON meta-tools lose
  schema validation and hurt accuracy; search->describe->execute with real
  schemas on the call path is the sound variant. Our scale (5 core + ~30/adapter)
  is past the threshold once two engines are enabled: a `find_engine_tool` /
  `call_engine_tool` pair, config-gated, is justified; below that, toolsets alone.
- **Composite tools beat granular ones**: Maestro collapsed ~30 imperative ops
  into `run(yaml)` + cheat_sheet; mobile-mcp added `mobile_batch_commands`.
  Adapter toolsets should expose flows, not 1:1 CLI verbs.
- Numbers to remember: ~500-700 tokens per tool definition; 5-7 tools per turn
  is the sweet spot; accuracy degrades past 30-50 tools; OpenAI hard cap 128.
- Notable admission: Playwright-MCP now recommends CLI+skills over MCP for
  coding agents (token efficiency); keep simslim/adb CLI reachable as escape hatch.

## 2. Go orchestrator core API shape

- **Concrete `Orchestrator` struct, never an interface** (add methods forever
  without breaking). Adapters stay small interfaces: `Platform` frozen at
  current 8 methods; future features = new small optional interfaces + flags.
- **Replace Optimizer type-assert with explicit capability discovery**:
  `Platform.Capabilities() CapabilitySet`; registry stores
  `PlatformInfo{Name, Capabilities}`. Type-assert becomes a documented
  `As`-style escape hatch only (gocloud precedent).
- **Registry must error on duplicate registration** (current one silently
  overwrites; gocloud URLMux panics - loud is correct).
- **Config at process boundary = struct; library knobs = fallible functional
  options** (`Option func(*config) error`).
- Package layout: `pkg/orchestrator` (Orchestrator, Registry, Options) /
  `pkg/contract` (frozen interfaces + DTOs) / `platforms/*` adapters /
  `pkg/mcp` thin transport. Add a Platform conformance test suite (gocloud's
  main interchangeability mechanism). Guard public API drift with apidiff in CI.
- **Remote adapter closes the two-machine loop**: `orchestrator.RemotePlatform`
  implementing `Platform` over JSON-RPC/HTTP against a `mcp-sim agent` on the
  secondary Mac. Same domain model both sides (Nomad pattern); no gRPC needed -
  stdlib HTTP + existing JSON DTOs already in pkg/contract.
- References: gocloud design doc + URL mux + As; database/sql driver pattern;
  docker/go client "return structs"; dave.cheney functional options;
  OTel-Go interface-evolution pain (otel-go#3920).

## 3. Engine landscape (ATD / simslim / remote Android)

- **ATD ceiling is API 33** (no aosp_atd for 34+; confirmed again in 2026 via
  cpython CI fallback to plain aosp). arm64-v8a images run natively under
  Hypervisor.framework on Apple Silicon. ~1.5-2.5 GB host RSS per instance
  (emulator still defaults hw.ramSize=2GB - set it explicitly at AVD create).
  ATD = system-image variant, not a new platform: Android adapter treats it as
  a boot profile (image tag + flags + ramSize), one control plane.
- **simslim is alone in its niche**, v0.6.x+, validated on Xcode 26.6 / iOS 26;
  mechanism is per-simulator launchd disable DB, persists across reboots; NOT
  carried by clone or erase; preserved by FS copies (how CI images are built).
  Orchestrator hook: "ensure slim" idempotent step post-create/clone + verify
  after erase/runtime updates (matches our benchmark finding #1).
- **Remote Android converges on adb-over-TCP**: redroid (Linux hosts, binder
  modules required; not viable in Docker Desktop/Dory VMs), cuttlefish (KVM,
  `cvd` REST orchestration API exists - google/cloud-android-orchestration).
  One `remote-android` adapter (host:port + adb connect) covers both plus
  emulator-over-network; create/destroy is the only per-backend difference.
- **CI parallel Android**: Gradle Managed Devices + ATD + snapshots is the
  Google-endorsed path; snapshots matter as much as image choice for
  throughput -> orchestrator boot options should include snapshot restore.
- **Screen state for agents: accessibility-tree first, screenshot fallback**
  (mobile-mcp via WDA/UIAutomator2; newer on-device HTTP servers expose
  uitree+screenshot). scrcpy v3+/4 raw stream = push scrcpy-server via adb,
  raw H.264 over TCP, change-driven frames. Minimal `stream_info` tool returns
  transport/host/port/codec/serial; orchestrator owns adb-forward port
  allocation and server lifecycle.
- Cross-cutting: all resource-efficiency paths = adb/simctl control plane +
  per-instance profile/slim hook + one TCP media endpoint. Validates adapter
  design; core needs boot-profile concept, port allocation, stream_info capability.

## Links to next steps

- Plan `.agents/plans/in-electric-swift.md` Phase 1 (extraction) should adopt
  the Capabilities() + concrete-struct + fallible-options API above.
- Phase 4 (tool extensibility) should adopt toolsets + config-gated
  discovery meta-tool; revisit tool-count threshold when a third engine lands.
