package bootstrap_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/espetro/mcp-sim/internal/bootstrap"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/orchestrator"
)

// disabledConfig is hermeticConfig with the agent-device verifier gated off,
// so maybeVerifier never probes the real binary.
func disabledVerifierConfig() config.Config {
	cfg := hermeticConfig()
	off := false
	cfg.Controllers.AgentDevice.Verifier = &off
	cfg.Controllers.AgentDevice.BinPath = "/nonexistent/agent-device-binary-xyz"
	return cfg
}

func TestMaybeVerifierExplicitlyDisabled(t *testing.T) {
	cfg := disabledVerifierConfig()
	v, err := bootstrap.MaybeVerifier(context.Background(), cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("maybeVerifier: %v", err)
	}
	if v != nil {
		t.Errorf("verifier should be nil when explicitly disabled, got %v", v.Name())
	}
}

func TestBuildOrchestratorWithoutVerifierDegrades(t *testing.T) {
	cfg := disabledVerifierConfig()
	logger := slog.New(slog.DiscardHandler)

	orch, err := bootstrap.BuildOrchestrator(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("BuildOrchestrator: %v", err)
	}
	if _, ok := orch.Verifier(); ok {
		t.Fatal("no verifier expected, got one")
	}
	// Verifier tools must degrade with a structured verifier_unavailable error.
	if err := orch.VerifyTap(context.Background(), "d1", "n1"); err == nil {
		t.Fatal("want verifier_unavailable error, got nil")
	} else {
		var te *orchestrator.ToolError
		if !errors.As(err, &te) || te.Code != "verifier_unavailable" {
			t.Errorf("want verifier_unavailable ToolError, got %v", err)
		}
	}
}
