# mcp-sim

MCP server for iOS Simulator and Android Emulator lifecycle management.

## Guides

- [Agent setup](agent-setup.md) — copyable setup spec for coding agents
- [Authentication](auth.md) — bearer token auth for the HTTP surface
- [Architecture](architecture.md) — layers and embedding the orchestrator
- [simslim integration](simslim.md) — optional iOS simulator memory slimming
- [Tailscale setup](tailscale.md) — running over Tailscale
- [Running as a service](service.md) — install as a native OS service (launchd/systemd/Windows Service)
- [Installing and launching apps](install.md) — install_app/launch_app, MCPSIM_ARTIFACT_ROOTS, artifact:// refs
- [Observability](observability.md) — OTel spans, JSONL audit log, MCPSIM_OBSERVABILITY_ENABLED
- [Verifier (device interaction)](verifier.md) — verifier_* tools, agent-device backend
- [Adding a platform](adding-platform.md) — implementing the Platform interface
