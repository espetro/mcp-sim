// Package paths resolves the mcp-sim persistence directory in a
// platform-idiomatic way: ~/.config/mcp-sim on darwin and linux (XDG-ish,
// matching the documented layout) and %AppData%\mcp-sim on Windows via
// os.UserConfigDir.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// Dir returns the directory where mcp-sim persists generated files (token,
// client snippet, default config). On darwin and linux it is
// $HOME/.config/mcp-sim; on windows it is os.UserConfigDir()/mcp-sim
// (typically %AppData%\mcp-sim).
func Dir() (string, error) {
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil || dir == "" {
			return "", err
		}
		return filepath.Join(dir, "mcp-sim"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", err
	}
	return filepath.Join(home, ".config", "mcp-sim"), nil
}
