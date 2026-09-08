# simslim integration (iOS memory slimming)

mcp-sim can reduce the memory footprint of booted iOS simulators by roughly
4x using [simslim](https://github.com/mobai-app/simslim), which disables the
launchd daemons a simulator does not need. This is fully optional and off by
default: some features (push notifications, StoreKit, Spotlight search) stop
working while a simulator is slimmed. Everything is reversible with
`simslim off`.

## How it works

mcp-sim shells out to the `simslim` CLI. It never imports the simslim Go
library, so mcp-sim keeps building on Windows and Linux. The integration is
wired through the optional `contract.Optimizer` interface: no new MCP tools
are added. Slimming happens automatically in two places:

- **`boot_device`** — when `slim.enabled` and `slim.on_boot` are set (or the
  caller passes `optimize: true`), the simulator is slimmed right after boot.
- **`wipe_device`** — erasing a simulator resets its launchd overrides to
  stock, so mcp-sim re-applies slimming after a successful wipe when
  `slim.on_boot` is set.

`get_state` includes an `optimizer` block when the platform has an optimizer
active:

```json
{
  "state": "running",
  "optimizer": {
    "slimmed": true,
    "persistent": true,
    "phys_footprint_bytes": 515000000,
    "process_count": 31
  }
}
```

On iOS runtimes older than 18.5 the runtime cannot persist launchd override
state, so mcp-sim falls back to `simslim on --no-reboot` (current boot session
only) and `get_state` surfaces a warning.

## Setup

```sh
brew install espetro/mcp-sim/mcp-sim mobai-app/tap/simslim
```

Then enable slim in `~/.config/mcp-sim/config.yaml`:

```yaml
platforms:
  ios:
    slim:
      enabled: true       # probe simslim on PATH (requires >= 0.6.0)
      on_boot: true       # slim each simulator during boot_device
      # profile: ""      # path to a profile JSON; default slim when empty
      # except: []       # category IDs to leave enabled (e.g. ["push", "store"])
      # keep: []         # individual launchd labels to keep running
      # boot_timeout: 10m   # SIMSLIM_BOOT_TIMEOUT passthrough
      # spawn_timeout: 2m   # SIMSLIM_SPAWN_TIMEOUT passthrough
```

Or via environment variables: `MCPSIM_IOS_SLIM_ENABLED`,
`MCPSIM_IOS_SLIM_ON_BOOT`, `MCPSIM_IOS_SLIM_PROFILE`,
`MCPSIM_IOS_SLIM_EXCEPT`, `MCPSIM_IOS_SLIM_KEEP`,
`MCPSIM_IOS_SLIM_BOOT_TIMEOUT`, `MCPSIM_IOS_SLIM_SPAWN_TIMEOUT`.

If `simslim` is missing or older than 0.6.0, mcp-sim logs a warning and
skips slimming; the server and all iOS tools still work.

## Per-boot override

`boot_device` accepts an optional `optimize` argument that overrides the
configured default for that call only:

- `optimize: true` — slim this boot even if `on_boot` is false
- `optimize: false` — boot stock even if `on_boot` is true
- omitted — use the config default

## What breaks without which category

Run `simslim profiles` to list daemon categories and the feature each one
backs. Common ones:

| Category | Disabled feature |
|---|---|
| `push` | Push notifications (APNs) |
| `store` | App Store / StoreKit |
| `search` | Spotlight search |

Use `slim.except` (category level) or `slim.keep` (single daemon level) to
keep what your tests need. `simslim doctor <udid> --requires <feature>`
checks whether a required feature still works on a slimmed simulator.

The simslim CLI stays user-reachable: agents and humans can run
`simslim status`, `simslim off <udid>`, or `simslim measure <udid> --json`
directly at any time.
