package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/internal/core"
	srv "github.com/espetro/mcp-sim/internal/http"
	applog "github.com/espetro/mcp-sim/internal/log"
	"github.com/espetro/mcp-sim/pkg/mcp"
	"github.com/espetro/mcp-sim/platforms/android"
	"github.com/espetro/mcp-sim/platforms/ios"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	// Simple flag parsing (no subcommand framework needed).
	if len(args) == 0 {
		return fmt.Errorf("usage: mcp-sim serve|mcp|version")
	}

	switch args[0] {
	case "version":
		fmt.Println("mcp-sim", version)
		return nil
	case "serve":
		return serve(args[1:])
	case "mcp":
		return mcpMode(args[1:])
	default:
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("mcp-sim serve", flag.ContinueOnError)
	listenAddr := fs.String("listen", "", "Address to bind (overrides config)")
	_ = fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if *listenAddr != "" {
		cfg.Server.Listen = *listenAddr
	}

	logger := applog.New(cfg.Server.LogLevel, cfg.Server.LogFormat)
	ctx := applog.WithContext(context.Background(), logger)

	registry := core.NewRegistry(logger)
	lifecycle := core.NewLifecycle(registry)

	// Register platforms.
	if cfg.Platforms.IOS.Enabled {
		iosPlatform, err := ios.New(ctx, cfg.Platforms.IOS)
		if err != nil {
			return fmt.Errorf("ios platform: %w", err)
		}
		registry.RegisterPlatform(iosPlatform)
	}
	if cfg.Platforms.Android.Enabled {
		androidPlatform, err := android.New(cfg.Platforms.Android)
		if err != nil {
			return fmt.Errorf("android platform: %w", err)
		}
		registry.RegisterPlatform(androidPlatform)
	}

	// Register controllers.
	if cfg.Controllers.AgentDevice.Enabled {
		registry.RegisterController(agentdevice.New(cfg.Controllers.AgentDevice))
	}

	mcpServer := mcp.NewServer(registry, lifecycle, logger)

	// Build HTTP handler.
	mux := http.NewServeMux()

	// MCP endpoint via streamable HTTP.
	mux.Handle("/mcp", mcpServer.StreamableHTTPHandler())

	// Health endpoint.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Wrap the mux so the internal server does not panic trying to re-register
	// /healthz; its own /healthz handler is functionally identical.
	httpServer := srv.New(cfg.Server.Listen, http.HandlerFunc(mux.ServeHTTP))

	logger.Info("mcp-sim starting", "addr", cfg.Server.Listen)

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutdown signal received")
		registry.ShutdownAll()
	}()

	return httpServer.ListenAndServe(ctx)
}

func mcpMode(args []string) error {
	fs := flag.NewFlagSet("mcp-sim mcp", flag.ContinueOnError)
	_ = fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := applog.New(cfg.Server.LogLevel, cfg.Server.LogFormat)
	ctx := applog.WithContext(context.Background(), logger)

	registry := core.NewRegistry(logger)
	lifecycle := core.NewLifecycle(registry)

	if cfg.Platforms.IOS.Enabled {
		iosPlatform, _ := ios.New(ctx, cfg.Platforms.IOS)
		if iosPlatform != nil {
			registry.RegisterPlatform(iosPlatform)
		}
	}
	if cfg.Platforms.Android.Enabled {
		androidPlatform, _ := android.New(cfg.Platforms.Android)
		if androidPlatform != nil {
			registry.RegisterPlatform(androidPlatform)
		}
	}
	if cfg.Controllers.AgentDevice.Enabled {
		registry.RegisterController(agentdevice.New(cfg.Controllers.AgentDevice))
	}

	mcpServer := mcp.NewServer(registry, lifecycle, logger)

	// Run with stdio transport.
	return mcpServer.Run(ctx, &sdkmcp.StdioTransport{})
}
