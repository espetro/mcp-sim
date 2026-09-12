# Tailscale deployment

`mcp-sim` is network-agnostic, and its HTTP surface is protected by bearer
auth. This page covers the network setup; the full auth model (token
resolution, first boot, client config, rotation) is in
[docs/auth.md](auth.md).

Over a tailnet, bearer auth is defense in depth: the tailnet already gives
you a private network and ACLs, and the token adds a second boundary on top.
Keep auth on unless you deliberately want the ACLs to be the only boundary.

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

If the ACLs are the only boundary you want, you can disable bearer auth with
an explicit trust acknowledgment:

```bash
mcp-sim serve --listen :9090 --insecure-no-auth
# refused: ":9090" is non-loopback, needs one of:
mcp-sim serve --listen :9090 --insecure-no-auth --insecure-no-auth-ack
# or:
MCPSIM_TRUSTED_NETWORK=true mcp-sim serve --listen :9090 --insecure-no-auth
```

## MCP client config (HTTP over the tailnet)

With auth on (default), add the bearer header. One-liner from another
machine, using ssh to fetch the token:

```bash
claude mcp add --transport http mcp-sim https://host:9090/mcp --header "Authorization: Bearer $(ssh host cat ~/.config/mcp-sim/token)"
```

Or paste the generated `~/.config/mcp-sim/client-snippet.json` into your
client's `mcpServers` block:

```json
{
  "mcpServers": {
    "mcp-sim": {
      "url": "http://<TAILSCALE_IP>:9090/mcp",
      "headers": {
        "Authorization": "Bearer <token>"
      }
    }
  }
}
```

See [docs/auth.md](auth.md) for token resolution and rotation.

For clients that launch mcp-sim as a subprocess instead (stdio):

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
