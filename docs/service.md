# Running mcp-sim as a service

`mcp-sim service` installs mcp-sim as a native background service, managed by
whichever OS it runs on:

- macOS → `launchd`
- Linux → `systemd` (or SysV/Upstart as fallback)
- Windows → Windows Service

## Install and manage

```bash
mcp-sim service install --listen :9090 --config ~/.config/mcp-sim/config.yaml
mcp-sim service start
mcp-sim service status
mcp-sim service stop
mcp-sim service restart
mcp-sim service uninstall
```

`--listen` and `--config` are only read at `install` time — they're recorded
as the arguments the OS service manager passes to the hidden
`mcp-sim service run` entry point it invokes on every start.

Installing system-wide requires elevated privileges (`sudo` on macOS/Linux, an
elevated shell on Windows). To skip that on macOS/Linux, install a per-user
service instead (LaunchAgent / `systemd --user`, no root needed):

```bash
mcp-sim service install --user --listen :9090
mcp-sim service start
```

Windows Service install always needs elevation — there's no per-user
equivalent on that OS.

## Manual launchd customization (macOS)

If you'd rather manage the launchd plist yourself instead of using
`mcp-sim service install`, create
`~/Library/LaunchAgents/com.espetro.mcp-sim.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.espetro.mcp-sim</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/mcp-sim</string>
    <string>serve</string>
    <string>--listen</string>
    <string>:9090</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/usr/local/var/log/mcp-sim.log</string>
  <key>StandardErrorPath</key>
  <string>/usr/local/var/log/mcp-sim.log</string>
</dict>
</plist>
```

Load the service:

```bash
mkdir -p /usr/local/var/log
launchctl load ~/Library/LaunchAgents/com.espetro.mcp-sim.plist
```

### Using Homebrew services

If installed via Homebrew:

```bash
brew services start mcp-sim
brew services stop mcp-sim
brew services restart mcp-sim
```

Logs: `brew services log mcp-sim`
