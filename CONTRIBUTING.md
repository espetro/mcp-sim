# Contributing to mcp-sim

## Separation of concerns (load-bearing rule)

mcp-sim owns **emulator lifecycle only** — power, state, data wipe, deep-link launch.

It does NOT ship UI automation, tap/click/screenshot tools, or verification logic.
That role is delegated to Controller adapters (e.g. agent-device).

If you add a platform adapter that also adds verification tools because "it was convenient",
that PR will be closed. The split is load-bearing.

Users wanting a different controller (XCTest, Maestro, custom) implement the Controller
interface and configure it in their install — no changes to platform adapters needed.

## PR checklist

- [ ] No new UI/verification tools added (search PR diff for tap, screenshot, view hierarchy, etc.)
- [ ] New platform adapter implements `pkg/contract.Platform` in full
- [ ] New platform adapter includes integration test gated on `MCPSIM_INTEGRATION=1`
- [ ] `task validate` passes locally
- [ ] CHANGELOG.md updated

## Development

```bash
# Install dependencies
go mod tidy

# Run tests
task test

# Lint
task lint

# Validate (typecheck + lint + test)
task validate

# Run locally
task run
```

## Commit style

Conventional commits. Each commit is atomic and self-contained.

## Branches

- `main`: protected, requires PR + CI green
- `develop`: integration branch, feature branches merge here
- `release/vX.Y.Z`: release branches
- `hotfix/vX.Y.Z`: emergency fixes from main
