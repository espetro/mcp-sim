package config

import (
	"testing"
)

func TestObservabilityDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Observability.Enabled {
		t.Error("observability should be disabled by default")
	}
	if cfg.Observability.AuditPath != "file" {
		t.Errorf("audit_path default = %q, want \"file\"", cfg.Observability.AuditPath)
	}
}

func TestObservabilityEnvOverrides(t *testing.T) {
	t.Setenv("MCPSIM_OBSERVABILITY_ENABLED", "true")
	t.Setenv("MCPSIM_AUDIT_LOG", "/tmp/mcp-sim-test/audit.jsonl")
	t.Setenv("MCPSIM_OTLP_ENDPOINT", "http://localhost:4318")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Observability.Enabled {
		t.Error("MCPSIM_OBSERVABILITY_ENABLED=true should enable observability")
	}
	if cfg.Observability.AuditPath != "/tmp/mcp-sim-test/audit.jsonl" {
		t.Errorf("audit_path = %q", cfg.Observability.AuditPath)
	}
	if cfg.Observability.OTLPEndpoint != "http://localhost:4318" {
		t.Errorf("otlp_endpoint = %q", cfg.Observability.OTLPEndpoint)
	}
}
