# macOS launchd service

## Using launchd

Create `~/Library/LaunchAgents/com.espetro.mcp-sim.plist`:

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

## Using Homebrew services

If installed via Homebrew:

```bash
brew services start mcp-sim
brew services stop mcp-sim
brew services restart mcp-sim
```

Logs: `brew services log mcp-sim`
