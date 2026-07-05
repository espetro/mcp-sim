package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/internal/core"
	srv "github.com/espetro/mcp-sim/internal/http"
	applog "github.com/espetro/mcp-sim/internal/log"
	"github.com/espetro/mcp-sim/internal/version"
	"github.com/espetro/mcp-sim/pkg/mcp"
	"github.com/espetro/mcp-sim/platforms/android"
	"github.com/espetro/mcp-sim/platforms/ios"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Subcommand names.
const (
	cmdHelp    = "help"
	cmdServe   = "serve"
	cmdMCP     = "mcp"
	cmdVersion = "version"
)

const usageRoot = `mcp-sim — MCP server for mobile emulator lifecycle

Usage:
  mcp-sim [command] [flags]

Commands:
  serve       Start the HTTP/SSE server (long-lived, default for service mode)
  mcp         Run over stdio (spawnable per agent session)
  version     Print version information
  help        Print this message

Run "mcp-sim <command> --help" for command-specific flags.
`

const usageServe = `mcp-sim serve — start the HTTP/SSE server

Usage:
  mcp-sim serve [flags]

Flags:
  -listen, --listen  Address to bind (default ":9090" or $MCPSIM_LISTEN)
  -config, --config  Path to YAML config file (default $MCPSIM_CONFIG or ~/.config/mcp-sim/config.yaml)
  -h, --help        Show this help

Endpoints when running:
  /mcp     MCP-over-streamable-HTTP transport
  /healthz Health check (returns "ok")
`

const usageMCP = `mcp-sim mcp — run over stdio

Usage:
  mcp-sim mcp [flags]

Flags:
  -config, --config  Path to YAML config file (default $MCPSIM_CONFIG or ~/.config/mcp-sim/config.yaml)
  -h, --help        Show this help

Reads JSON-RPC from stdin, writes to stdout.
`

func main() {
	if err := run(os.Args[0], os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is the testable entry point; stdout is injected for help output.
func run(prog string, args []string, stdout io.Writer) error {
	// No args, --help, -h, or "help" all show root usage.
	if len(args) == 0 || isHelpFlag(args[0]) || args[0] == cmdHelp {
		fmt.Fprint(stdout, usageRoot)
		return nil
	}

	switch args[0] {
	case cmdVersion:
		return printVersion(stdout)
	case cmdServe:
		return runServe(prog, args[1:], stdout)
	case cmdMCP:
		return runMCP(prog, args[1:], stdout)
	case "-v", "--version":
		return printVersion(stdout)
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n\n", prog, args[0])
		fmt.Fprint(stdout, usageRoot)
		os.Exit(2)
		return nil // unreachable
	}
}

func isHelpFlag(s string) bool {
	return s == "-h" || s == "--help" || s == "-help"
}

func printVersion(stdout io.Writer) error {
	fmt.Fprintln(stdout, version.Human())
	return nil
}

// runServe parses serve-specific flags and starts the HTTP server.
func runServe(prog string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet(cmdServe, flag.ContinueOnError)
	fs.SetOutput(stdout)
	listenAddr := fs.String("listen", "", "Address to bind (overrides config)")
	configPath := fs.String("config", "", "Path to YAML config file")
	fs.Usage = func() { fmt.Fprint(stdout, usageServe) }
	// Re-prepend the program name so ContinueOnError prints "serve -h".
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		// If user passed -h/--help anywhere, flag package's ContinueOnError
		// returns ErrHelp, which we already handled. Anything else: show help.
		fmt.Fprint(stdout, usageServe)
		return err
	}
	// Detect trailing --help that flag.Parse consumed.
	for _, a := range args {
		if isHelpFlag(a) {
			fmt.Fprint(stdout, usageServe)
			return nil
		}
	}
	_ = listenAddr
	_ = configPath
	// Dispatch to the existing handler.
	return serveImpl(prog, *listenAddr, *configPath)
}

// runMCP parses mcp-specific flags and starts the stdio server.
func runMCP(prog string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet(cmdMCP, flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", "", "Path to YAML config file")
	fs.Usage = func() { fmt.Fprint(stdout, usageMCP) }
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		fmt.Fprint(stdout, usageMCP)
		return err
	}
	for _, a := range args {
		if isHelpFlag(a) {
			fmt.Fprint(stdout, usageMCP)
			return nil
		}
	}
	return mcpImpl(prog, *configPath)
}

// serveImpl and mcpImpl are the actual implementation of each subcommand.
// Extracted so run() stays small and command flags parse first.
func serveImpl(prog, listenAddr, configPath string) error {
	_ = configPath // TODO: pass through to config.Load; env override preserved
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if listenAddr != "" {
		cfg.Server.Listen = listenAddr
	}

	logger := applog.New(cfg.Server.LogLevel, cfg.Server.LogFormat)
	ctx := applog.WithContext(context.Background(), logger)

	registry := core.NewRegistry(logger)
	lifecycle := core.NewLifecycle(registry)

	if cfg.Platforms.IOS.Enabled {
		iosPlatform, err := ios.New(ctx, cfg.Platforms.IOS)
		if err != nil {
			return fmt.Errorf("ios platform: %w", err)
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
			return fmt.Errorf("android platform: %w", err)
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

	logger.Info("mcp-sim starting",
		"version", version.Version,
		"addr", cfg.Server.Listen,
		"platforms", platformNames(registry),
		"controllers", controllerNames(registry))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutdown signal received")
		registry.ShutdownAll()
	}()

	if err := httpServer.ListenAndServe(ctx); err != nil {
		return err
	}
	return nil
}

func mcpImpl(prog, configPath string) error {
	_ = configPath
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
	return mcpServer.Run(ctx, &sdkmcp.StdioTransport{})
}

// platformNames returns a sorted list of registered platform names.
func platformNames(r *core.Registry) []string {
	ps := r.AllPlatforms()
	names := make([]string, 0, len(ps))
	for name := range ps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func controllerNames(r *core.Registry) []string {
	cs := r.AllControllers()
	names := make([]string, 0, len(cs))
	for name := range cs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
