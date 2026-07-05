package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/bootstrap"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/internal/core"
	applog "github.com/espetro/mcp-sim/internal/log"
	svc "github.com/espetro/mcp-sim/internal/service"
	"github.com/espetro/mcp-sim/internal/version"
	"github.com/espetro/mcp-sim/pkg/mcp"
	"github.com/espetro/mcp-sim/platforms/android"
	"github.com/espetro/mcp-sim/platforms/ios"

	kservice "github.com/kardianos/service"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Subcommand names.
const (
	cmdHelp    = "help"
	cmdServe   = "serve"
	cmdMCP     = "mcp"
	cmdService = "service"
	cmdVersion = "version"
)

const usageRoot = `mcp-sim — MCP server for mobile emulator lifecycle

Usage:
  mcp-sim [command] [flags]

Commands:
  serve       Start the HTTP/SSE server (long-lived, default for service mode)
  mcp         Run over stdio (spawnable per agent session)
  service     Install/manage mcp-sim as a native OS service
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

const usageService = `mcp-sim service — install/manage mcp-sim as a native OS service

Installs mcp-sim as a launchd service (macOS), systemd service (Linux), or
Windows Service (Windows), managed by the OS the same way as any other
background service.

Usage:
  mcp-sim service <action> [flags]

Actions:
  install     Register mcp-sim with the OS service manager
  uninstall   Remove mcp-sim from the OS service manager
  start       Start the installed service
  stop        Stop the installed service
  restart     Restart the installed service
  status      Print the installed service's status
  run         Hidden entry point invoked by the OS service manager itself

Flags (only meaningful with "install"):
  -listen, --listen  Address to bind (overrides config; carried to "run")
  -config, --config  Path to YAML config file (carried to "run")
  -user, --user      Install as a per-user service, no root required
                      (unsupported on Windows)
  -h, --help        Show this help
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
	case cmdService:
		return runService(prog, args[1:], stdout)
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

// runService parses the service action and flags, then dispatches to
// kardianos/service's control functions or runs as the installed service.
func runService(prog string, args []string, stdout io.Writer) error {
	if len(args) == 0 || isHelpFlag(args[0]) || args[0] == cmdHelp {
		fmt.Fprint(stdout, usageService)
		return nil
	}
	action := args[0]

	fs := flag.NewFlagSet(cmdService, flag.ContinueOnError)
	fs.SetOutput(stdout)
	listenAddr := fs.String("listen", "", "Address to bind (overrides config)")
	configPath := fs.String("config", "", "Path to YAML config file")
	userService := fs.Bool("user", false, "Install as a per-user service (no root required; unsupported on Windows)")
	fs.Usage = func() { fmt.Fprint(stdout, usageService) }
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		fmt.Fprint(stdout, usageService)
		return err
	}

	var svcArgs []string
	if *listenAddr != "" {
		svcArgs = append(svcArgs, "--listen", *listenAddr)
	}
	if *configPath != "" {
		svcArgs = append(svcArgs, "--config", *configPath)
	}

	svcConfig, err := svc.BuildConfig(svcArgs, *userService)
	if err != nil {
		return fmt.Errorf("building service config: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if *listenAddr != "" {
		cfg.Server.Listen = *listenAddr
	}

	logger := applog.New(cfg.Server.LogLevel, cfg.Server.LogFormat)
	program := svc.NewProgram(cfg, logger)

	s, err := kservice.New(program, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	switch action {
	case "run":
		return s.Run()
	case "status":
		status, err := s.Status()
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, serviceStatusString(status))
		return nil
	case "install", "uninstall", "start", "stop", "restart":
		if err := kservice.Control(s, action); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "mcp-sim service: %s ok\n", action)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown service action %q\n\n", prog, action)
		fmt.Fprint(stdout, usageService)
		os.Exit(2)
		return nil // unreachable
	}
}

func serviceStatusString(status kservice.Status) string {
	switch status {
	case kservice.StatusRunning:
		return "running"
	case kservice.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
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

	registry, httpServer, err := bootstrap.BuildHTTPServer(ctx, cfg, logger)
	if err != nil {
		return err
	}

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
