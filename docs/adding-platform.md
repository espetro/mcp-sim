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

## Key rules

- Do NOT add verification tools (tap, screenshot, getTree) — those belong in Controllers
- Implement `AwaitReady` for a good developer experience
- For long-lived spawned processes, set process attrs via a package-private `setProcAttr(cmd *exec.Cmd)` helper split across `procattr_unix.go` (`//go:build !windows`, `Setpgid:true`) and `procattr_windows.go` (`//go:build windows`, `CreationFlags: CREATE_NEW_PROCESS_GROUP`) — see `platforms/android/procattr_*.go`. `Setpgid` is Unix-only; any platform adapter targeting Windows needs this split.
- Add integration tests gated on `MCPSIM_INTEGRATION=1`

## Testing

```bash
MCPSIM_INTEGRATION=1 go test ./platforms/myplatform/...
```
