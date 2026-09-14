package agentdevice

import (
	"fmt"
	"os/exec"
)

// HintText is the user-facing setup hint when the binary is missing.
const HintText = "install agent-device (macOS: brew install callstack/tap/agent-device), or set bin_path / MCPSIM_AGENT_DEVICE_BIN"

// DetectBinary resolves the agent-device binary: an explicit path wins,
// otherwise $PATH is searched. Returns "" when not found.
func DetectBinary(binPath string) string {
	if binPath != "" {
		if path, err := exec.LookPath(binPath); err == nil {
			return path
		}
		return ""
	}
	path, err := exec.LookPath("agent-device")
	if err != nil {
		return ""
	}
	return path
}

// EnsureInstalled reports a setup hint when the binary is missing. The
// auto-install flag from the original plan was dropped: mcp-sim should never
// shell out to a package manager on the user's behalf without explicit
// consent. The hint names the exact command instead.
func EnsureInstalled(binPath string) (string, error) {
	path := DetectBinary(binPath)
	if path == "" {
		return "", fmt.Errorf("agent-device binary not found (%s)", HintText)
	}
	return path, nil
}
