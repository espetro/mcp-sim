package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/espetro/mcp-sim/internal/version"
	"github.com/espetro/mcp-sim/pkg/contract"
	"github.com/espetro/mcp-sim/pkg/orchestrator"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server with mcp-sim tools.
type Server struct {
	impl   *mcp.Server
	logger *slog.Logger
}

// NewServer creates an MCP server with the mcp-sim tool set registered.
// Every tool maps 1:1 onto an orchestrator method.
func NewServer(orch *orchestrator.Orchestrator, logger *slog.Logger) *Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-sim",
		Title:   "MCP Simulator Server",
		Version: version.Version,
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
		devs, err := orch.List(ctx)
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
		Optimize *bool  `json:"optimize,omitempty"`
	}) (*mcp.CallToolResult, contract.Device, error) {
		opts := contract.StartOpts{
			NoWindow: in.NoWindow,
			Port:     in.Port,
			Timeout:  time.Duration(in.Timeout) * time.Second,
			Optimize: in.Optimize,
		}
		dev, err := orch.Boot(ctx, in.Platform, in.Target, opts)
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
		if err := orch.Stop(ctx, in.Platform, in.Target); err != nil {
			return nil, contract.Device{}, err
		}
		state, err := orch.State(ctx, in.Platform, in.Target)
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
		if err := orch.Wipe(ctx, in.Platform, in.Target); err != nil {
			return nil, contract.Device{}, err
		}
		state, err := orch.State(ctx, in.Platform, in.Target)
		return nil, contract.Device{Platform: in.Platform, ID: in.Target, State: state}, err
	})

	// get_state
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_state",
		Description: "Get the current state of a device (stopped/booting/running/error).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
	}) (*mcp.CallToolResult, struct {
		State     string                  `json:"state"`
		Optimizer *GetStateOptimizerBlock `json:"optimizer,omitempty"`
	}, error) {
		state, err := orch.State(ctx, in.Platform, in.Target)
		if err != nil {
			return nil, struct {
				State     string                  `json:"state"`
				Optimizer *GetStateOptimizerBlock `json:"optimizer,omitempty"`
			}{State: string(state)}, err
		}
		out := struct {
			State     string                  `json:"state"`
			Optimizer *GetStateOptimizerBlock `json:"optimizer,omitempty"`
		}{State: string(state)}
		if st, usage, err := orch.OptimizerState(ctx, in.Platform, in.Target); err == nil && st != nil {
			out.Optimizer = &GetStateOptimizerBlock{
				Slimmed:    st.Slimmed,
				Persistent: st.Persistent,
			}
			if !st.Persistent && st.Slimmed {
				out.Optimizer.Warning = "runtime cannot persist launchd overrides; state reverts to stock at next reboot"
			}
			if usage != nil {
				out.Optimizer.PhysFootprintBytes = usage.PhysFootprintBytes
				out.Optimizer.ProcessCount = usage.ProcessCount
			}
		}
		return nil, out, nil
	})

	// await_ready
	mcp.AddTool(s, &mcp.Tool{
		Name:        "await_ready",
		Description: "Block until the device is fully booted, or the timeout fires.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
		Timeout  int    `json:"timeout,omitempty"`
	}) (*mcp.CallToolResult, struct{ Ready bool }, error) {
		timeout := time.Duration(in.Timeout) * time.Second
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		if err := orch.AwaitReady(ctx, in.Platform, in.Target, timeout); err != nil {
			return nil, struct{ Ready bool }{}, err
		}
		return nil, struct{ Ready bool }{Ready: true}, nil
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
		if err := orch.OpenURL(ctx, in.Platform, in.Target, in.URL); err != nil {
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
		info, err := orch.StartController(ctx, in.Name, contract.StartConfig{Port: in.Port})
		return nil, info, err
	})

	// stop_controller
	mcp.AddTool(s, &mcp.Tool{
		Name:        "stop_controller",
		Description: "Stop a controller proxy daemon.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Name string `json:"name"`
	}) (*mcp.CallToolResult, contract.ProxyInfo, error) {
		if err := orch.StopController(ctx, in.Name); err != nil {
			return nil, contract.ProxyInfo{}, err
		}
		info, err := orch.ControllerStatus(ctx, in.Name)
		return nil, info, err
	})

	// stream_info
	mcp.AddTool(s, &mcp.Tool{
		Name:        "stream_info",
		Description: "Get on demand GUI mirroring guidance for a device (scrcpy over adb TCP/IP on Android).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Platform string `json:"platform"`
		Target   string `json:"target"`
	}) (*mcp.CallToolResult, struct {
		Mirroring string `json:"mirroring,omitempty"`
		Command   string `json:"command,omitempty"`
		Supported bool   `json:"supported"`
		Reason    string `json:"reason,omitempty"`
	}, error) {
		if in.Platform != "android" {
			return nil, struct {
				Mirroring string `json:"mirroring,omitempty"`
				Command   string `json:"command,omitempty"`
				Supported bool   `json:"supported"`
				Reason    string `json:"reason,omitempty"`
			}{Reason: "stream_info only supports the android platform"}, fmt.Errorf("unsupported platform %q", in.Platform)
		}
		state, err := orch.State(ctx, "android", in.Target)
		if err != nil {
			return nil, struct {
				Mirroring string `json:"mirroring,omitempty"`
				Command   string `json:"command,omitempty"`
				Supported bool   `json:"supported"`
				Reason    string `json:"reason,omitempty"`
			}{}, err
		}
		if state != contract.DeviceStateRunning {
			return nil, struct {
				Mirroring string `json:"mirroring,omitempty"`
				Command   string `json:"command,omitempty"`
				Supported bool   `json:"supported"`
				Reason    string `json:"reason,omitempty"`
			}{Reason: "device is not running"}, nil
		}
		return nil, struct {
			Mirroring string `json:"mirroring,omitempty"`
			Command   string `json:"command,omitempty"`
			Supported bool   `json:"supported"`
			Reason    string `json:"reason,omitempty"`
		}{
			Mirroring: "scrcpy",
			Supported: true,
			Command: "Use `adb devices` to find the serial for the AVD named " + in.Target +
				", then: `adb -s <serial> tcpip 5555`, `adb connect <host>:5555`, " +
				"`scrcpy --tcpip=<host>:5555`. " +
				"For screenshots without mirroring (and on ATD images, where scrcpy is unreliable): " +
				"`adb -s <serial> exec-out screencap -p > screen.png`.",
		}, nil
	})

	// controller_status
	mcp.AddTool(s, &mcp.Tool{
		Name:        "controller_status",
		Description: "Get the status of a controller proxy daemon.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Name string `json:"name"`
	}) (*mcp.CallToolResult, contract.ProxyInfo, error) {
		info, err := orch.ControllerStatus(ctx, in.Name)
		return nil, info, err
	})

	return &Server{impl: s, logger: logger}
}

// GetStateOptimizerBlock is the optional optimizer section of get_state output.
type GetStateOptimizerBlock struct {
	Slimmed            bool   `json:"slimmed"`
	Persistent         bool   `json:"persistent"`
	Warning            string `json:"warning,omitempty"`
	PhysFootprintBytes int64  `json:"phys_footprint_bytes,omitempty"`
	ProcessCount       int    `json:"process_count,omitempty"`
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
