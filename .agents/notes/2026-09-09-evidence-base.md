# Evidence base: orchestrator-core research verdict (2026-09-09)

Sources backing each claim in `.agents/notes/2026-09-09-orchestrator-research.md`.
Compiled for handoff; next session should not need to re-research.

## Local primary sources (verified hands-on)

| Source | Backs |
|---|---|
| `bench/results/2026-09-09.{json,md}`, `bench/journal.md` | memory 2.6-4.0x reduction; boot-cost numbers; seam 15/15; MCP-vs-CLI payload/timing table; erase-persistence invariant |
| `bench/bench-slim/main.go`, `mcpvcli.go` | benchmark method (phys_footprint via simslim measure --json; real MCP client vs raw simctl) |
| `pkg/contract/platform.go` (Optimizer), `internal/core/registry.go` (silent-overwrite registry) | current-API-shape facts in the Go-API recommendation |
| `simslim` CLI 0.8.0 installed locally + cached source at `~/.opensrc/repos/github.com/mobai-app/simslim/main` (output.go, slim.go, measure.go, cmd/simslim) | JSON shapes, `on/off/status/measure/verify` verbs, iOS <18.5 --no-reboot semantics, launchd-override persistence claims |
| Manual verification in this session: slim -> shutdown -> erase -> boot = still slim (iOS 26.5) | erase-does-NOT-reset finding |
| Session `~/.local/state/maki/sessions/CeHdXa3LxbSz95mFvo3az.jsonl` + plan `~/.local/state/maki/plans/possible-equipped-dove.md` | dreaming prior art (OpenClaw native dreaming, Icattj 4-phase auto-dream, MicroClaw/RayClaw reflectors) |

## Web sources (retrieved 2026-09-09 via research subagents)

### Tool-surface extensibility
- https://github.com/microsoft/playwright-mcp - --caps capability gating; core ~22 tools; README now recommends CLI+skills over MCP for coding agents (token efficiency admission)
- https://github.com/googleapis/mcp-toolbox - toolsets (named groups), per-toolset endpoints, skills-generate
- MCP spec + client best practices (2026-07-28 revision): https://modelcontextprotocol.io/docs/2026-07-28/develop/clients/client-best-practices - subscriptions opt-in; endorses stable call_tool dispatcher for cache stability
- Codex list_changed broken: https://github.com/openai/codex/issues/33266, /37417
- Claude Code list_changed partial: https://github.com/anthropics/claude-code/issues/66084
- Anthropic Advanced Tool Use (Tool Search Tool GA, defer_loading): https://www.anthropic.com/engineering/advanced-tool-use - 72K->500 tokens; accuracy 49->74% (Opus 4), 79.5->88.1% (Opus 4.5); >10 tools threshold guidance
- https://aicost.tools/blog/mcp-context-tax-tool-search/ and https://tanayshah.dev/blog/anthropic-tool-search-deferred-loading/ - token-per-tool costs (GitHub 35 tools ~ 26K tokens; 5-7 active tools sweet spot; accuracy 41-83% at ~200 tools); Speakeasy search->describe->execute (160x schema reduction, +1 turn)
- https://github.com/mobile-next/mobile-mcp - ~34 tools flat, a11y-first, mobile_batch_commands
- https://docs.maestro.dev/get-started/maestro-mcp - 9 tools; run(yaml) composite pattern

### Go library design
- gocloud design doc: https://github.com/google/go-cloud/blob/master/internal/docs/design.md - portable type + driver; conformance tests
- gocloud As concept: https://gocloud.dev/concepts/as/ ; URL mux: https://gocloud.dev/concepts/urls/ + https://pkg.go.dev/gocloud.dev/blob
- https://abhinavg.net/2022/12/06/designing-go-libraries/ and https://0x46.net/thoughts/2018/12/29/go-libraries/ - return structs not interfaces; concrete types evolve
- Nomad architecture: https://developer.hashicorp.com/nomad/docs/architecture - servers/clients planes, driver capability attributes
- Functional options: https://dave.cheney.net/2014/10/17/functional-options-for-friendly-api-s ; https://www.followtheprocess.codes/posts/functional-options/ ; https://www.bytesizego.com/blog/10-years-functional-options-golang - fallible options consensus
- Interface evolution pain: https://github.com/open-telemetry/opentelemetry-go/issues/3920 ; https://github.com/golang/go/issues/61447
- Config structs at boundaries: https://dev.to/gabrielanhaia/functional-options-vs-builder-vs-config-struct-in-go-pick-one-3i1c

### Engine landscape
- ATD images 30-33, no API 34/35 ATD: https://developer.android.com/studio/test/managed-devices ; cpython CI fallback commit https://github.com/hugovk/cpython/commit/a95ee3a21d97afdbe6bd2ce4cd8343a36cd13b02
- ATD constraints: https://stackoverflow.com/questions/79040976
- GitHub Actions declined ATD preinstall: https://github.com/actions/runner-images/issues/8676
- simslim: https://github.com/MobAI-App/simslim ; 4.0GB->0.9GB, 5->19 sims on 16GB: https://mobai.run/blog/19-ios-simulators-on-a-16gb-mac
- Cuttlefish: https://source.android.com/docs/devices/cuttlefish ; multi-tenancy: https://source.android.com/docs/devices/cuttlefish/multi-tenancy ; HTTP orchestration: https://github.com/google/cloud-android-orchestration
- redroid: https://github.com/remote-android/redroid-doc (binder modules; arm64 Linux)
- scrcpy raw stream (server push, H.264 over TCP, change-driven): https://github.com/Genymobile/scrcpy/blob/master/doc/develop.md ; https://github.com/Genymobile/scrcpy/pull/2971 ; https://github.com/genymobile/scrcpy/issues/6212

## Confidence notes

- High (primary/verified): simslim behaviors, benchmark numbers, MCP-vs-CLI measurements, current repo API shape, dreaming prior art.
- Medium (official docs, no hands-on): Playwright caps details, Toolbox toolsets, gocloud patterns, Nomad, Cuttlefish orchestration API.
- Lower (third-party/uncorroborated): token-cost figures from blog posts (aicost.tools, tanayshah.dev - directionally consistent with Anthropic's own numbers); MicroClaw/RayClaw RAM estimates; whether microclaw anthropic provider honors llm_base_url (flagged open in the VPS plan too).
- Re-verify at build time: ATD image availability per API level via sdkmanager on the host; scrcpy server flags against installed scrcpy version.
