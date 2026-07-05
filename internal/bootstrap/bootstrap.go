// Package bootstrap wires the registry, MCP server, and HTTP handler shared
// by the "serve" subcommand and the OS-native service program.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/internal/core"
	srv "github.com/espetro/mcp-sim/internal/http"
	"github.com/espetro/mcp-sim/pkg/mcp"
	"github.com/espetro/mcp-sim/platforms/android"
	"github.com/espetro/mcp-sim/platforms/ios"
)

// BuildHTTPServer constructs the registry and HTTP server for serve/service modes.
// It registers whichever platform/controller adapters are enabled and detected.
func BuildHTTPServer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*core.Registry, *srv.Server, error) {
	registry := core.NewRegistry(logger)
	lifecycle := core.NewLifecycle(registry)

	if cfg.Platforms.IOS.Enabled {
		iosPlatform, err := ios.New(ctx, cfg.Platforms.IOS)
		if err != nil {
			return nil, nil, fmt.Errorf("ios platform: %w", err)
		}
		if iosPlatform == nil {
			logger.Warn("ios platform disabled — Xcode/xcrun not detected, skipping iOS tools")
		} else {
			registry.RegisterPlatform(iosPlatform)
		}
	}
	if cfg.Platforms.Android.Enabled {
		androidPlatform, err := android.New(cfg.Platforms.Android)
		if err != nil {
			return nil, nil, fmt.Errorf("android platform: %w", err)
		}
		if androidPlatform == nil {
			logger.Warn("android platform disabled — emulator/adb not detected, skipping Android tools")
		} else {
			registry.RegisterPlatform(androidPlatform)
		}
	}
	if cfg.Controllers.AgentDevice.Enabled {
		registry.RegisterController(agentdevice.New(cfg.Controllers.AgentDevice))
	}

	mcpServer := mcp.NewServer(registry, lifecycle, logger)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpServer.StreamableHTTPHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	httpServer := srv.New(cfg.Server.Listen, http.HandlerFunc(mux.ServeHTTP))
	return registry, httpServer, nil
}
