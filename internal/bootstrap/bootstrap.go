// Package bootstrap wires the orchestrator, MCP server, and HTTP handler
// shared by the "serve", "mcp", and service modes.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/config"
	srv "github.com/espetro/mcp-sim/internal/http"
	"github.com/espetro/mcp-sim/pkg/mcp"
	"github.com/espetro/mcp-sim/pkg/orchestrator"
	"github.com/espetro/mcp-sim/platforms/android"
	"github.com/espetro/mcp-sim/platforms/ios"
)

// BuildOrchestrator builds the orchestrator with whichever platform and
// controller adapters are enabled and detected, logging what got registered.
// Adapters returning (nil, nil) (toolchain absent) are skipped with a warning;
// duplicate-name errors cannot happen here and propagate as unexpected.
func BuildOrchestrator(ctx context.Context, cfg config.Config, logger *slog.Logger) (*orchestrator.Orchestrator, error) {
	var opts []orchestrator.Option
	opts = append(opts, orchestrator.WithLogger(logger))

	var platformNames, controllerNames []string

	if cfg.Platforms.IOS.Enabled {
		iosPlatform, err := ios.New(ctx, cfg.Platforms.IOS)
		if err != nil {
			return nil, fmt.Errorf("ios platform: %w", err)
		}
		if iosPlatform == nil {
			logger.Warn("ios platform disabled — Xcode/xcrun not detected, skipping iOS tools")
		} else {
			opts = append(opts, orchestrator.WithPlatform(iosPlatform))
			platformNames = append(platformNames, iosPlatform.Name())
		}
	}
	if cfg.Platforms.Android.Enabled {
		androidPlatform, err := android.New(cfg.Platforms.Android)
		if err != nil {
			return nil, fmt.Errorf("android platform: %w", err)
		}
		if androidPlatform == nil {
			logger.Warn("android platform disabled — emulator/adb not detected, skipping Android tools")
		} else {
			opts = append(opts, orchestrator.WithPlatform(androidPlatform))
			platformNames = append(platformNames, androidPlatform.Name())
		}
	}
	if cfg.Controllers.AgentDevice.Enabled {
		c := agentdevice.New(cfg.Controllers.AgentDevice)
		opts = append(opts, orchestrator.WithController(c))
		controllerNames = append(controllerNames, c.Name())
	}

	orch, err := orchestrator.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("building orchestrator: %w", err)
	}

	sort.Strings(platformNames)
	sort.Strings(controllerNames)
	logger.Info("orchestrator ready",
		"platforms", platformNames,
		"controllers", controllerNames)
	return orch, nil
}

// BuildHTTPServer constructs the orchestrator and HTTP server for
// serve/service modes, keeping the /healthz and /mcp mux wiring.
func BuildHTTPServer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*orchestrator.Orchestrator, *srv.Server, error) {
	orch, err := BuildOrchestrator(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}

	mcpServer := mcp.NewServer(orch, logger)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpServer.StreamableHTTPHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	httpServer := srv.New(cfg.Server.Listen, http.HandlerFunc(mux.ServeHTTP))
	return orch, httpServer, nil
}
