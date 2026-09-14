// Package agentdevice implements contract.Verifier on top of the agent-device
// CLI, spawned as a child MCP server over stdio (`agent-device mcp`).
//
// The adapter is a thin dispatcher: every Verifier method maps onto one
// agent-device MCP tool call. All tap/long-press/type operations take a ref
// from a prior Snapshot call so the caller never deals with raw coordinates.
package agentdevice

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/espetro/mcp-sim/pkg/contract"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// verifierName is the backend identifier reported by Verifier.Name.
const verifierName = "agent-device"

// pingTimeout bounds the Ping health check.
const pingTimeout = 5 * time.Second

// Verifier implements contract.Verifier by dispatching to a child
// `agent-device mcp` process over stdio.
type Verifier struct {
	binPath string
	session *sdkmcp.ClientSession
}

// compile-time conformance check.
var _ contract.Verifier = (*Verifier)(nil)

// NewVerifier spawns `bin mcp` as a child stdio MCP server, performs the MCP
// handshake, and returns a ready Verifier. Call Close to shut the child down.
func NewVerifier(ctx context.Context, binPath string) (*Verifier, error) {
	// #nosec G204 -- binPath is resolved via exec.LookPath by the caller
	cmd := exec.CommandContext(ctx, binPath, "mcp")
	client := sdkmcp.NewClient(&sdkmcp.Implementation{
		Name:    "mcp-sim",
		Version: "1.0.0",
	}, nil)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("agent-device: starting %s mcp: %w", binPath, err)
	}
	return &Verifier{binPath: binPath, session: session}, nil
}

// Close terminates the child agent-device process.
func (v *Verifier) Close() error {
	return v.session.Close()
}

// Name returns the backend identifier.
func (v *Verifier) Name() string { return verifierName }

// Ping checks the child server is responsive.
func (v *Verifier) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return v.session.Ping(ctx, nil)
}

// call invokes one agent-device tool by name with the given arguments.
func (v *Verifier) call(ctx context.Context, tool string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	res, err := v.session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      tool,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("agent-device: %s: %w", tool, err)
	}
	if res.IsError {
		return nil, fmt.Errorf("agent-device: %s: %s", tool, toolErrorText(res))
	}
	return res, nil
}

// deviceArgs builds the common per-call argument set.
func deviceArgs(target string) map[string]any {
	return map[string]any{"device": target}
}
