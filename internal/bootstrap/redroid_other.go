//go:build !linux

package bootstrap

import (
	"context"
	"log/slog"

	"github.com/espetro/mcp-sim/pkg/orchestrator"
)

// maybeRegisterRedroid is a no-op off Linux: Redroid requires Linux binder
// kernel modules unavailable in Docker VMs on macOS/Windows (docs/redroid.md).
func maybeRegisterRedroid(_ context.Context, _ *[]orchestrator.Option, _ *[]string, _ *slog.Logger) error {
	return nil
}
