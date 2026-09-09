package orchestrator

import "github.com/espetro/mcp-sim/pkg/contract"

// ToolError wraps an error with a stable MCP error code. pkg/mcp maps Code to
// MCP ToolResultError codes; codes are the contract.Err* constants.
type ToolError struct {
	Code string
	Msg  string
}

// NewToolError builds a ToolError.
func NewToolError(code, msg string) *ToolError {
	return &ToolError{Code: code, Msg: msg}
}

func (e *ToolError) Error() string { return e.Msg }

func unsupportedPlatform(name string) error {
	return NewToolError(contract.ErrUnsupportedPlatform, "platform not found: "+name)
}

func unsupportedController(name string) error {
	return NewToolError(contract.ErrUnsupportedController, "controller not found: "+name)
}
