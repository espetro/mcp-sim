// Package bootstrap wires the orchestrator, MCP server, and HTTP handler
// shared by the "serve", "mcp", and service modes.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/auth"
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
	if err := maybeRegisterRedroid(ctx, &opts, &platformNames, logger); err != nil {
		return nil, fmt.Errorf("redroid platform: %w", err)
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
// serve/service modes, keeping the /healthz and /mcp mux wiring. Auth is
// resolved from the config (env > server.auth.token > generated token file)
// and enforced on /mcp when cfg.Server.Auth.Enabled.
func BuildHTTPServer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*orchestrator.Orchestrator, *srv.Server, error) {
	var token string
	if cfg.Server.Auth.Enabled {
		var err error
		token, err = resolveToken(cfg, logger)
		if err != nil {
			return nil, nil, err
		}
	}
	return BuildHTTPServerWithAuth(ctx, cfg, logger, token)
}

// BuildHTTPServerWithAuth is BuildHTTPServer with an explicit bearer token.
// An empty token disables auth regardless of cfg.Server.Auth.Enabled.
func BuildHTTPServerWithAuth(ctx context.Context, cfg config.Config, logger *slog.Logger, token string) (*orchestrator.Orchestrator, *srv.Server, error) {
	orch, err := BuildOrchestrator(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}

	mcpServer := mcp.NewServer(orch, logger)

	handler := http.Handler(mcpServer.StreamableHTTPHandler())
	if cfg.Server.Auth.Enabled && token != "" {
		handler = auth.Middleware(token, "http://"+listenHostPort(cfg.Server.Listen)+auth.MetadataPath)(handler)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	// RFC 9728 protected resource metadata is served unauthenticated so
	// spec-aware clients can discover the Bearer requirement.
	resource := "http://" + listenHostPort(cfg.Server.Listen) + "/mcp"
	mux.Handle(auth.MetadataPath, auth.MetadataHandler(resource))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	httpServer := srv.New(cfg.Server.Listen, http.HandlerFunc(mux.ServeHTTP))
	return orch, httpServer, nil
}

// resolveToken resolves the bearer token by precedence: MCPSIM_AUTH_TOKEN >
// server.auth.token > generated token file. Generated tokens are persisted
// to tokenPath (never rotated) with a client-snippet.json next to them; the
// raw token is only printed when stderr is an interactive TTY.
func resolveToken(cfg config.Config, logger *slog.Logger) (string, error) {
	envToken := os.Getenv("MCPSIM_AUTH_TOKEN")
	token, source := auth.Resolve(envToken, cfg.Server.Auth.Token)
	tokenPath := DefaultTokenPath()
	snippetPath := DefaultSnippetPath()

	if source != auth.SourceGenerated {
		logger.Info("auth token resolved", "source", string(source), "token_path", "")
		return token, nil
	}

	generated, err := auth.LoadOrCreate(tokenPath)
	if err != nil {
		return "", fmt.Errorf("loading auth token: %w", err)
	}
	url := "http://" + listenHostPort(cfg.Server.Listen) + "/mcp"
	if err := auth.WriteSnippet(snippetPath, url, generated); err != nil {
		return "", fmt.Errorf("writing client snippet: %w", err)
	}
	logger.Info("auth token ready", "token_path", tokenPath, "snippet_path", snippetPath)
	if isTerminal(os.Stderr) {
		fmt.Fprintf(os.Stderr, "\nmcp-sim bearer token (first boot, saved to %s):\n\n  %s\n\nClient config (%s):\n\n", tokenPath, generated, snippetPath)
		if data, err := os.ReadFile(snippetPath); err == nil {
			fmt.Fprintln(os.Stderr, string(data))
		}
	}
	return generated, nil
}

// DefaultTokenPath is where the generated bearer token persists.
func DefaultTokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "token"
	}
	return filepath.Join(home, ".config", "mcp-sim", "token")
}

// DefaultSnippetPath is where the ready-to-paste client snippet is written.
func DefaultSnippetPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "client-snippet.json"
	}
	return filepath.Join(home, ".config", "mcp-sim", "client-snippet.json")
}

// isTerminal reports whether f is an interactive character device.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// listenHostPort returns the host:port form of a listen address.
func listenHostPort(listen string) string {
	if !strings.Contains(listen, ":") {
		return listen
	}
	if strings.HasPrefix(listen, ":") {
		return "localhost" + listen
	}
	return listen
}
