# install_app / launch_app (2026-09-14)

- Tool dispatch lives in pkg/mcp/server.go as typed AddTool closures over
  orchestrator methods; there is no internal/orchestrator. Platform adapters
  are platforms/ (not internal/platforms).
- Optional platform abilities follow the Optimizer pattern: interface in
  pkg/contract/platform.go, capability bit in capabilities.go, CapabilitiesFor
  derives bits via type assertion; orchestrator enforces the bit before
  contract.As[T] dispatch.
- Android adb serial: use p.serialFor (port map -> emulator-N, else raw target).
- Bootstrap HTTP test drives a full MCP handshake over httptest; notifications
  return 202 so jsonRPCCall (fatals on non-200) cannot be used for them.
- Real-boot simtest smoke for install/launch is follow-up: no internal/simtest
  harness exists on develop (the test/integration-actual-boot branch has none
  either beyond platform conformance tests).
