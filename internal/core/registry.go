package core

import (
	"context"
	"log/slog"
	"sync"

	"github.com/espetro/mcp-sim/pkg/contract"
)

// Registry holds registered platforms and controllers.
type Registry struct {
	mu          sync.RWMutex
	platforms   map[string]contract.Platform
	controllers map[string]contract.Controller
	logger      *slog.Logger
}

// NewRegistry creates a new registry.
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		platforms:   make(map[string]contract.Platform),
		controllers: make(map[string]contract.Controller),
		logger:      logger,
	}
}

// RegisterPlatform adds a platform to the registry.
func (r *Registry) RegisterPlatform(p contract.Platform) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.platforms[p.Name()] = p
	r.logger.Info("platform registered", "name", p.Name())
}

// RegisterController adds a controller to the registry.
func (r *Registry) RegisterController(c contract.Controller) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.controllers[c.Name()] = c
	r.logger.Info("controller registered", "name", c.Name())
}

// PlatformByName returns a platform by name.
func (r *Registry) PlatformByName(name string) (contract.Platform, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.platforms[name]
	return p, ok
}

// ControllerByName returns a controller by name.
func (r *Registry) ControllerByName(name string) (contract.Controller, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.controllers[name]
	return c, ok
}

// AllPlatforms returns all registered platforms.
func (r *Registry) AllPlatforms() map[string]contract.Platform {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]contract.Platform, len(r.platforms))
	for k, v := range r.platforms {
		result[k] = v
	}
	return result
}

// AllControllers returns all registered controllers.
func (r *Registry) AllControllers() map[string]contract.Controller {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]contract.Controller, len(r.controllers))
	for k, v := range r.controllers {
		result[k] = v
	}
	return result
}

// ShutdownAll stops all running devices and controllers.
func (r *Registry) ShutdownAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ctx := context.Background()
	for _, p := range r.platforms {
		devs, _ := p.List(ctx)
		for _, d := range devs {
			if d.State == contract.DeviceStateRunning {
				_ = p.Stop(ctx, d.ID)
			}
		}
	}
	for _, c := range r.controllers {
		_ = c.Stop(ctx)
	}
}
