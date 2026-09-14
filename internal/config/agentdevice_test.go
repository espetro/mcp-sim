package config

import (
	"testing"
)

func TestAgentDeviceVerifierDefaultsNil(t *testing.T) {
	t.Parallel()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Controllers.AgentDevice.Verifier != nil {
		t.Errorf("verifier default should be nil (binary-detection decides), got %v", *cfg.Controllers.AgentDevice.Verifier)
	}
	if cfg.Controllers.AgentDevice.BinPath != "" {
		t.Errorf("bin_path default should be empty, got %q", cfg.Controllers.AgentDevice.BinPath)
	}
}

func TestAgentDeviceVerifierEnvOverride(t *testing.T) {
	t.Setenv("MCPSIM_AGENT_DEVICE_VERIFIER", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Controllers.AgentDevice.Verifier == nil || *cfg.Controllers.AgentDevice.Verifier {
		t.Error("MCPSIM_AGENT_DEVICE_VERIFIER=false should disable the verifier explicitly")
	}
}

func TestAgentDeviceVerifierEnvOverrideTrue(t *testing.T) {
	t.Setenv("MCPSIM_AGENT_DEVICE_VERIFIER", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Controllers.AgentDevice.Verifier == nil || !*cfg.Controllers.AgentDevice.Verifier {
		t.Error("MCPSIM_AGENT_DEVICE_VERIFIER=true should enable the verifier explicitly")
	}
}

func TestAgentDeviceBinPathEnvOverride(t *testing.T) {
	t.Setenv("MCPSIM_AGENT_DEVICE_BIN", "/usr/local/bin/agent-device")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Controllers.AgentDevice.BinPath != "/usr/local/bin/agent-device" {
		t.Errorf("bin_path = %q", cfg.Controllers.AgentDevice.BinPath)
	}
}
