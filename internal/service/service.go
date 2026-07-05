// Package service wires mcp-sim into a native OS-managed background service
// (launchd on macOS, systemd on Linux, Windows Service on Windows) via
// github.com/kardianos/service.
package service

import (
	"context"
	"log/slog"
	"os"

	"github.com/espetro/mcp-sim/internal/bootstrap"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/internal/core"

	kservice "github.com/kardianos/service"
)

// Program implements kservice.Interface, wrapping the same HTTP server
// construction used by the "serve" subcommand.
type Program struct {
	cfg    config.Config
	logger *slog.Logger

	registry *core.Registry
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewProgram creates a service program for the given config.
func NewProgram(cfg config.Config, logger *slog.Logger) *Program {
	return &Program{cfg: cfg, logger: logger}
}

// Start is invoked by the OS service manager. It must return quickly, so the
// HTTP server runs in a background goroutine.
func (p *Program) Start(s kservice.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	registry, httpServer, err := bootstrap.BuildHTTPServer(ctx, p.cfg, p.logger)
	if err != nil {
		cancel()
		return err
	}
	p.registry = registry
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)
		if err := httpServer.ListenAndServe(ctx); err != nil {
			p.logger.Error("http server stopped with error", "error", err)
		}
	}()
	return nil
}

// Stop is invoked by the OS service manager for graceful shutdown.
func (p *Program) Stop(s kservice.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		<-p.done
	}
	if p.registry != nil {
		p.registry.ShutdownAll()
	}
	return nil
}

// BuildConfig builds the kardianos/service Config used to install mcp-sim as
// a native OS service. args are extra CLI flags carried through to the
// hidden "service run" entry point the installed service actually invokes.
// When user is true, installs a per-user service (LaunchAgent / systemd
// --user) that doesn't require root — unsupported on Windows, where a
// Windows Service always needs an elevated install regardless.
func BuildConfig(args []string, user bool) (*kservice.Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &kservice.Config{
		Name:        "mcp-sim",
		DisplayName: "mcp-sim",
		Description: "MCP server for mobile emulator lifecycle",
		Executable:  exe,
		Arguments:   append([]string{"service", "run"}, args...),
		Option: kservice.KeyValue{
			"UserService": user,
		},
	}, nil
}
