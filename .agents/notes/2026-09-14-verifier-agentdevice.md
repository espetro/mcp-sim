# Verifier interface + agent-device adapter (2026-09-14)

- The repo pattern for optional abilities is the Optimizer pattern: interface
  in pkg/contract, capability bit in capabilities.go, CapabilitiesFor derives
  via type assertion, orchestrator method enforces the bit then dispatches
  contract.As[T]. The verifier deliberately does NOT follow it: agent-device
  drives devices across platforms (ios + android), so it is not a per-Platform
  extension. It is a separate singleton contract registered via
  orchestrator.WithVerifier and stored on the Orchestrator struct.
- MCP go-sdk v1.6 AddTool handlers must use the 3-value form
  (*CallToolResult, Out, error); the 2-value (Out, error) form infers Out
  incorrectly and fails to compile. Structured output comes back from the
  client as map[string]any with float64 numbers.
- Config fields added in a single edit can silently vanish if the editor
  applies edits against a stale buffer; go build catches it, but prefer
  scripted (python) edits for multi-block changes in this repo.
- Graceful degrade: verifier_* tools are always advertised; orchestrator
  returns ToolError code verifier_unavailable (contract.ErrVerifierUnavailable)
  when no backend registered. Test count assertion in
  internal/bootstrap/bootstrap_http_test.go (wantTools) must list every tool.
- Real-device simtest smoke for verifier_* is follow-up: no simtest harness on
  develop; agent-device binary not present on the dev machine at the time.
