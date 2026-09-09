# Architecture decision record: Orchestrator Core

Contributor-facing decision record for the Orchestrator Core extraction
(branch `feat/orchestrator-core`, plan
`.agents/plans/2026-09-09-orchestrator-core.md`). This file explains *why*,
not *how to use*. The user-facing overview lives in `docs/architecture.md`
(hosted); it is intentionally light and does not duplicate this content.

Research inputs (do not re-derive):
- `.agents/notes/2026-09-09-orchestrator-research.md` (Go API shape, tool surfaces, engine landscape)
- `.agents/notes/2026-09-09-simslim-benchmarks.md` and `bench/results/2026-09-09.md` (MCP vs CLI numbers, erase persistence invariant)
- `.agents/notes/2026-09-09-evidence-base.md` (source URLs and confidence levels)
- `.agents/drafts/2026-09-09-orchestrator-assessment.md` (what carries over, risks)

## Target layout

```
pkg/contract/        frozen adapter interfaces + DTOs + CapabilitySet (public)
pkg/orchestrator/    Orchestrator, Registry, Options, wipe/profile policy, conformance suite (public)
pkg/mcp/             thin transport: tool schema -> orchestrator method, nothing else
platforms/*          adapters (behavior unchanged; gain Capabilities())
controllers/*        unchanged
internal/bootstrap/  single BuildOrchestrator used by serve, mcp, and service modes
internal/core/       deleted
```

`internal/core` disappears; its registry, lifecycle, and tool-handler logic
moves into `pkg/orchestrator` as methods on one concrete type. If a soft
landing is needed for one release, a deprecated shim re-exporting
`pkg/orchestrator` is acceptable, then removed.

## Decision 1: concrete struct, not interface, for Orchestrator

`Orchestrator` is a concrete struct: `func New(opts ...Option) (*Orchestrator, error)`.
Callers depend on the struct directly. We never extract an `Orchestrator`
interface.

Rationale: adding a method to a concrete type is never a breaking change.
Adding a method to an interface that third parties implement is always a
breaking change. The docker/go client settled on returning structs for
exactly this reason. The cautionary tale is OpenTelemetry Go: evolving
widely-implemented interfaces across v1 to v2 caused years of migration pain
(otel-go issue 3920). Since `pkg/orchestrator` becomes a public, embeddable
library, concrete-struct is the only shape that lets the API grow indefinitely.

The seams that *do* need interfaces stay narrow and frozen: `Platform` and
`Controller` in `pkg/contract`, because adapters are third-party
extensibility points, unlike the orchestrator itself.

## Decision 2: Platform frozen at 8 methods; capabilities for everything else

`contract.Platform` freezes at its current 8 methods (`List`, `Start`,
`Stop`, `State`, `AwaitReady`, `Wipe`, `OpenURL`, plus `Capabilities` as
added by this work). New capability never means a new method on `Platform`.
It means:

1. `Platform.Capabilities() CapabilitySet` advertises support. `CapabilitySet`
   is a small `uint32` bitmask with named constants (`CapList`, `CapStart`,
   `CapStop`, `CapState`, `CapAwaitReady`, `CapWipe`, `CapOpenURL`,
   `CapOptimize`, `CapMeasure`; future: `CapStream`, `CapSnapshot`). Cheap to
   store, compare, and serialize.
2. Small optional interfaces (for example the existing `Optimizer`) carry any
   richer method surface, advertised via the capability set.

The registry dispatches on capabilities, not type assertions. The type
assert survives only as a documented escape hatch in the gocloud `As` style:
a helper `contract.As[T](p Platform) (T, bool)` plus docs. Rationale: gocloud
faced the same problem (many backends, uneven feature support) and capability
plus optional-interface is the proven evolution path; bare type asserts in
dispatch logic silently break when adapters reorder.

## Decision 3: registry errors on duplicate registration

`RegisterPlatform` (and `RegisterController`, and the `WithPlatform` /
`WithController` options) return an error when a platform of the same name
is already registered. Today the registry silently overwrites.

Rationale: silent overwrite hides wiring bugs (two adapters for one platform
name, config loaded twice). Loud is correct. gocloud's URLMux *panics* on
duplicate registration; since we are a library, we return an error instead of
panicking, but the strictness matches.

## Decision 4: config structs at the boundary, fallible options for the library

Two config regimes, deliberately separate:

- **Process boundary** (CLI, env vars, YAML file in `internal/config`):
  structs, resolved CLI > env > file > defaults, as today. Structs are the
  right shape here because YAML and env need stable, documentable keys.
- **Library knobs** (anything passed to `orchestrator.New`):
  fallible functional options, `type Option func(*config) error`
  (`WithLogger`, `WithPlatform`, `WithController`, later `WithWipePolicy`).
  Fallible because registration can fail (Decision 3), and the dave.cheney
  `Option func(*config) error` pattern makes that expressible without panics
  or a two-phase constructor.

Never leak YAML/env parsing into `pkg/orchestrator`; the library takes Go
values only.

## Decision 5: wipe is reconciliation, not erase

Semantics: **`Wipe` returns the device to its configured baseline.** The
invariant is `Wipe(); State()` converges to the same answer as immediately
after initial provisioning.

Mechanically, wipe is a `WipePolicy` owned by the orchestrator (not by
adapters): after `Platform.Wipe`, the orchestrator re-runs the platform's
profile hook. For iOS with slim enabled, that means erase then `Optimize`
(idempotent, benchmarked at roughly one minute). With no profile configured,
wipe is a plain erase, which is today's behavior.

Why: slim state survives `simctl erase` on iOS 18.5 and later, so a bare
erase is *not* a reset to known state. This generalizes: the base simulator
has the same drift class (runtime updates, launchd override persistence).
So the core owns reconciliation, not just command dispatch.

`Boot` is the universal convergence point: the same ensure-profile step runs
on boot, idempotently. Booting an already-provisioned device must not
trigger a redundant optimize call.

## Decision 6: availability semantics

- **Reads** (`List`, `State`): highly available and eventually consistent.
  They aggregate across platforms, tolerate partial adapter failure, and
  always return a coherent answer: partial results with per-platform error
  annotation rather than a total failure.
- **Actions** (`Boot`, `Stop`, `Wipe`, `Optimize`): best-effort and
  idempotent. Boot on a booted device reports success and convergence
  (replacing the raw simctl exit 149 error path, normalized at the core).
  Stop on a stopped device is a no-op success. Wipe twice produces the same
  result as wiping once. Never error on "already in target state".

## Dependency rule

- `pkg/orchestrator` imports only `pkg/contract` and the standard library.
- `pkg/mcp` imports `pkg/orchestrator` and is a thin transport: tool schema
  to orchestrator method, nothing else. No lifecycle logic there.
- Adapters (`platforms/*`, `controllers/*`) import `pkg/contract`.
- No import cycles. No `internal/` imports from public packages.

This keeps `pkg/orchestrator` embeddable by any Go program with zero
transport or config baggage, and makes `internal/bootstrap` the single place
where config structs turn into an `Orchestrator`.

## Conformance and API stability

- `pkg/orchestrator/conformance` provides `RunConformanceTests(t, factory)`
  covering lifecycle idempotency, wipe convergence, and capability honesty
  (advertised caps actually work). Every in-tree adapter runs it (iOS behind
  the usual macOS/Xcode guard). This is gocloud's interchangeability
  mechanism applied here.
- CI runs an apidiff check on `pkg/orchestrator` and `pkg/contract` to catch
  public API drift.
- `bench/bench-slim` gains an orchestrator-level smoke
  (boot then state then wipe then converge) as a perf gate.

## Docs placement rule

- **User docs** live in `docs/`, hosted at espetro.github.io/mcp-sim, built
  with docmd by `.github/workflows/docs.yml`, deployed from `main` only.
  Anything a user or their agent needs to install, configure, embed, or
  operate mcp-sim goes here.
- **Contributor docs** live in `.agents/docs/` (committed, never hosted):
  this decision record plus internal screens and maps.
- `docs/architecture.md` keeps only the light user-level overview and may
  link this file; content is not duplicated.

## Relation to `.agents/docs/screens/`

This record supersedes `.agents/docs/screens/` where they overlap (the
adapter model and lifecycle flows are restated here in decision form). The
screens files remain the detailed flow and config-surface maps and stay
authoritative for that; they are not deleted, and should be pruned only when
clearly stale after the extraction lands.
