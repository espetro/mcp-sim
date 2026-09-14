package bootstrap

import (
	"context"
	"log/slog"

	"github.com/espetro/mcp-sim/controllers/agentdevice"
	"github.com/espetro/mcp-sim/internal/config"
	"github.com/espetro/mcp-sim/pkg/contract"
)

// MaybeVerifier builds the agent-device Verifier when the configuration and
// the environment allow it, returning (nil, nil) when the verifier is
// disabled or the binary is absent. The MCP surface keeps advertising the
// verifier_* tools either way; the orchestrator answers them with a
// verifier_unavailable ToolError when no backend registered.
//
// Gating precedence (CLI > env > file > default):
//
//  1. cfg.Controllers.AgentDevice.Verifier set (env MCPSIM_AGENT_DEVICE_VERIFIER
//     or yaml controllers.agentdevice.verifier) — explicit on/off.
//  2. Unset: enabled only when the agent-device binary is detected (install
//     by default where available, silent no-op elsewhere).
func MaybeVerifier(ctx context.Context, cfg config.Config, logger *slog.Logger) (contract.Verifier, error) {
	ac := cfg.Controllers.AgentDevice
	if ac.Verifier != nil && !*ac.Verifier {
		logger.Debug("agent-device verifier disabled by config")
		return nil, nil
	}
	binPath := agentdevice.DetectBinary(ac.BinPath)
	if binPath == "" {
		if ac.Verifier != nil && *ac.Verifier {
			logger.Warn("agent-device verifier enabled but binary not found; " + agentdevice.HintText)
		} else {
			logger.Info("agent-device verifier not enabled (binary not found)")
		}
		return nil, nil
	}
	v, err := agentdevice.NewVerifier(ctx, binPath)
	if err != nil {
		return nil, err
	}
	return v, nil
}
