package contract

import (
	"context"
)

// Controller abstracts controller-adapter lifecycle (e.g. agent-device proxy).
type Controller interface {
	// Name returns the controller identifier (e.g. "agentdevice").
	Name() string
	// Start launches the controller proxy daemon.
	Start(ctx context.Context, cfg StartConfig) (ProxyInfo, error)
	// Stop terminates the controller proxy daemon.
	Stop(ctx context.Context) error
	// Status returns the current proxy state.
	Status(ctx context.Context) (ProxyInfo, error)
}
