//go:build linux

package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/espetro/mcp-sim/pkg/orchestrator"
	"github.com/espetro/mcp-sim/platforms/redroid"
)

// maybeRegisterRedroid registers the Redroid platform adapter when explicitly
// enabled via MCPSIM_REDROID_ENABLED and docker is available. Redroid is WIP
// (see docs/redroid.md) and Linux only, so it is opt in even on Linux.
func maybeRegisterRedroid(ctx context.Context, opts *[]orchestrator.Option, platformNames *[]string, logger *slog.Logger) error {
	if !truthy(os.Getenv("MCPSIM_REDROID_ENABLED")) {
		return nil
	}
	p, err := redroid.New()
	if err != nil {
		return err
	}
	if p == nil {
		logger.Warn("redroid platform enabled but docker not detected, skipping redroid tools")
		return nil
	}
	*opts = append(*opts, orchestrator.WithPlatform(p))
	*platformNames = append(*platformNames, p.Name())
	return nil
}

// truthy parses common boolean-ish env values.
func truthy(v string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false
	}
	return b
}
