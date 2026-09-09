# Adding a platform adapter

This guide walks through implementing the `contract.Platform` interface for a new emulator type.

## Step 1: Create the adapter package

```bash
mkdir -p platforms/myplatform
```

## Step 2: Implement contract.Platform

```go
package myplatform

type Platform struct{}

func New(cfg config.MyPlatformConfig) (*Platform, error) { ... }
func (p *Platform) Name() string { return "myplatform" }
func (p *Platform) List(ctx context.Context) ([]contract.Device, error) { ... }
func (p *Platform) Start(ctx context.Context, target string, opts contract.StartOpts) (contract.Device, error) { ... }
func (p *Platform) Stop(ctx context.Context, target string) error { ... }
func (p *Platform) State(ctx context.Context, target string) (contract.DeviceState, error) { ... }
func (p *Platform) AwaitReady(ctx context.Context, target string, timeout time.Duration) error { ... }
func (p *Platform) Wipe(ctx context.Context, target string) error { ... }
func (p *Platform) OpenURL(ctx context.Context, target, url string) error { ... }
```

## Step 3: Register in main.go

In `cmd/mcp-sim/main.go`, add to `serve()` and `mcpMode()`:

```go
if cfg.Platforms.MyPlatform.Enabled {
    registry.RegisterPlatform(myplatform.New(cfg.Platforms.MyPlatform))
}
```

## Step 4: Add config fields

In `internal/config/config.go`, add `MyPlatformConfig` and wire it up.

## Capabilities

`Platform` requires a `Capabilities() contract.CapabilitySet` method returning a `uint32` bitmask of supported operations:

```go
func (p *Platform) Capabilities() contract.CapabilitySet {
	s := contract.CapAll // list|start|stop|state|await_ready|wipe|open_url
	// Advertise CapOptimize/CapMeasure only if you also implement contract.Optimizer.
	if p.optimizer != nil {
		s = s.Enable(contract.CapOptimize).Enable(contract.CapMeasure)
	}
	return s
}
```

Rules:

- Base caps: `CapList`, `CapStart`, `CapStop`, `CapState`, `CapAwaitReady`, `CapWipe`, `CapOpenURL` (bundled as `contract.CapAll`).
- Advertise `CapOptimize`/`CapMeasure` only when the adapter actually implements `contract.Optimizer` — callers gate on both.
- Use `Has`/`Enable` for set operations; `String()` gives a human-readable listing for logs and error messages.
- For platform-specific behavior not in the contract, use the gocloud-style escape hatch `contract.As[T](p)` (a typed assertion) rather than widening `Platform`.

## Key rules

- Do NOT add verification tools (tap, screenshot, getTree) — those belong in Controllers
- Implement `AwaitReady` for a good developer experience
- For long-lived spawned processes, set process attrs via a package-private `setProcAttr(cmd *exec.Cmd)` helper split across `procattr_unix.go` (`//go:build !windows`, `Setpgid:true`) and `procattr_windows.go` (`//go:build windows`, `CreationFlags: CREATE_NEW_PROCESS_GROUP`) — see `platforms/android/procattr_*.go`. `Setpgid` is Unix-only; any platform adapter targeting Windows needs this split.
- Add integration tests gated on `MCPSIM_INTEGRATION=1`

## Testing

```bash
MCPSIM_INTEGRATION=1 go test ./platforms/myplatform/...
```
