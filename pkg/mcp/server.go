package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/espetro/mcp-sim/internal/core"
	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server with mcp-sim tools.
type Server struct {
	impl   *mcp.Server
	logger *slog.Logger
}

// NewServer creates an MCP server with the mcp-sim tool set registered.
func NewServer(registry *core.Registry, lifecycle *core.Lifecycle, logger *slog.Logger) *Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-sim",
		Title:   "MCP Simulator Server",
		Version: "dev",
	}, &mcp.ServerOptions{
		Instructions: "Mobile emulator/simulator control server.",
	})

	// list_devices
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_devices",
		Description: "List all available emulators and simulators across configured platforms.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, struct {
		Devices []contract.Device `json:"devices"`
	}, error) {
		devs, err := core.ListDevices(ctx, registry)
		return nil, struct {
			Devices []contract.Device `json:"devices"`
		}{Devices: devs}, err
	})

	// boot_device
	mcp.AddTool(s, &mcp.Tool{
		Name:        "boot_device",
		Description: "Boot a device by platform and target identifier.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
		NoWindow bool   `json:"no_window,omitempty"`
		Port     int    `json:"port,omitempty"`
		Timeout  int    `json:"timeout,omitempty"`
	}) (*mcp.CallToolResult, contract.Device, error) {
		opts := contract.StartOpts{
			NoWindow: in.NoWindow,
			Port:     in.Port,
			Timeout:  time.Duration(in.Timeout) * time.Second,
		}
		dev, err := lifecycle.BootDevice(ctx, in.Platform, in.Target, opts)
		return nil, dev, err
	})

	// stop_device
	mcp.AddTool(s, &mcp.Tool{
		Name:        "stop_device",
		Description: "Stop a running device by platform and target identifier.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
	}) (*mcp.CallToolResult, contract.Device, error) {
		if err := lifecycle.StopDevice(ctx, in.Platform, in.Target); err != nil {
			return nil, contract.Device{}, err
		}
		state, err := core.GetDeviceState(ctx, registry, in.Platform, in.Target)
		return nil, contract.Device{Platform: in.Platform, ID: in.Target, State: state}, err
	})

	// wipe_device
	mcp.AddTool(s, &mcp.Tool{
		Name:        "wipe_device",
		Description: "Wipe a device, erasing its user data. Stops the device first if needed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
	}) (*mcp.CallToolResult, contract.Device, error) {
		if err := lifecycle.WipeDevice(ctx, in.Platform, in.Target); err != nil {
			return nil, contract.Device{}, err
		}
		state, err := core.GetDeviceState(ctx, registry, in.Platform, in.Target)
		return nil, contract.Device{Platform: in.Platform, ID: in.Target, State: state}, err
	})

	// open_url
	mcp.AddTool(s, &mcp.Tool{
		Name:        "open_url",
		Description: "Open a URL or deep link on a device.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
		URL      string `json:"url"`
	}) (*mcp.CallToolResult, struct{ Success bool }, error) {
		if err := lifecycle.OpenURL(ctx, in.Platform, in.Target, in.URL); err != nil {
			return nil, struct{ Success bool }{}, err
		}
		return nil, struct{ Success bool }{Success: true}, nil
	})

	// start_controller
	mcp.AddTool(s, &mcp.Tool{
		Name:        "start_controller",
		Description: "Start a controller proxy daemon.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Name string `json:"name"`
		Port int    `json:"port,omitempty"`
	}) (*mcp.CallToolResult, contract.ProxyInfo, error) {
		info, err := core.StartController(ctx, registry, in.Name, contract.StartConfig{Port: in.Port})
		return nil, info, err
	})

	// stop_controller
	mcp.AddTool(s, &mcp.Tool{
		Name:        "stop_controller",
		Description: "Stop a controller proxy daemon.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Name string `json:"name"`
	}) (*mcp.CallToolResult, contract.ProxyInfo, error) {
		if err := core.StopController(ctx, registry, in.Name); err != nil {
			return nil, contract.ProxyInfo{}, err
		}
		info, err := core.ControllerStatus(ctx, registry, in.Name)
		return nil, info, err
	})

	// controller_status
	mcp.AddTool(s, &mcp.Tool{
		Name:        "controller_status",
		Description: "Get the status of a controller proxy daemon.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Name string `json:"name"`
	}) (*mcp.CallToolResult, contract.ProxyInfo, error) {
		info, err := core.ControllerStatus(ctx, registry, in.Name)
		return nil, info, err
	})

	return &Server{impl: s, logger: logger}
}

// Run runs the server over the given transport (e.g. stdio).
func (s *Server) Run(ctx context.Context, t mcp.Transport) error {
	return s.impl.Run(ctx, t)
}

// StreamableHTTPHandler returns an HTTP handler for the streamable MCP transport.
func (s *Server) StreamableHTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.impl
	}, nil)
}
