# Memory process

Project-scoped agent memory with time-bounded recall. All memory lives inside
this repo (no cross-project contamination). Modeled on the openclaw "dreaming"
idea: memories have a freshness half-life, old material is compacted or
dropped, and recent, load-bearing knowledge stays hot.

## Layout

| Path | Committed | Purpose | Compaction |
|---|---|---|---|
| `.agents/notes/` | yes | Durable findings worth recalling across sessions (benchmarks, semantics discoveries, decisions) | Prune notes superseded by newer ones; merge series into one dated note |
| `.agents/docs/` | yes | Permanent documentation (guides, runbooks) | Never auto-compacted; curated like code |
| `.agents/docs/screens/` | yes | User-flow documentation, two layers: product-level flows and low-level config/TUI maps (mirrors brioso's screens registry) | Refreshed on UI/config change; stale entries get deleted, not archived |
| `.agents/drafts/` | no (gitignored) | Raw intermediate research; a private DB of prior investigation | **Auto-compact candidate #1**: files older than ~30 days and not referenced by notes/docs get summarized into a dated `drafts/_archive/<month>.md` or dropped |
| `.agents/plans/` | no (gitignored) | Implementation plans (one per work item) | Same treatment as drafts once the plan is shipped |
| `AGENTS.md` | yes | The entrypoint: points the next agent at this file and the layout above | Curated |

## Dreaming (compaction policy)

There is no automatic dreamer running today; the process is agent-executed:

1. **On session start**: the agent reads `AGENTS.md` -> this file -> skims
   `.agents/notes/` (newest first) for anything touching the session's task.
2. **On session end** (or before context compaction): the agent writes any
   durable finding as a dated note in `.agents/notes/YYYY-MM-DD-<topic>.md`
   and, when a finding invalidates an older note, prepends a line to the old
   note: `Superseded by <new note>.`
3. **Periodic dream** (when notes exceed ~15 files or at each milestone):
   - merge same-topic note series into a single topic note,
   - delete notes fully superseded more than one milestone back,
   - compress `.agents/drafts/` older than 30 days into
     `drafts/_archive/<YYYY-MM>.md` (one paragraph per file) or drop,
   - update this file's log below.

## Log

- 2026-09-09: layout created. Seed notes: simslim benchmark findings,
  erase/persistence semantics, MCP-vs-CLI payload comparison.
