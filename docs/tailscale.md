# Tailscale deployment

mcp-sim is network-agnostic — it binds to all interfaces by default.

## Option 1: bind to Tailscale IP

On Mac A (the emulator host), find your Tailscale IP:

```bash
tailscale ip -4
```

Start mcp-sim:

```bash
mcp-sim serve --listen <TAILSCALE_IP>:9090
```

## Option 2: bind to all interfaces + Tailscale ACLs

```bash
mcp-sim serve --listen :9090
```

Configure Tailscale ACLs to allow your dev machine access to port 9090 on Mac A.

## MCP client config

```json
{
  "mcpServers": {
    "mcp-sim": {
      "command": "mcp-sim",
      "args": ["serve", "--listen", "<TAILSCALE_IP>:9090"]
    }
  }
}
```

## Tailscale ACL example

In your `tailscale ACL` policy:

```json
{
  "acls": [
    {
      "action": "accept",
      "src": ["<DEV_MACHINE_TAG>"],
      "dst": ["<MAC_A_TAG>:9090"]
    }
  ]
}
```
