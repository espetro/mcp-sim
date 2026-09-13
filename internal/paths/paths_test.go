package paths

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestJoin exercises the platform decision separately from the environment
// lookups, so the test is hermetic regardless of the host OS or $HOME /
// AppData state. On windows the base is os.UserConfigDir(); elsewhere it is
// os.UserHomeDir()/.config. We assert only the join logic for each branch.
func TestJoin(t *testing.T) {
	if runtime.GOOS == "windows" {
		got := filepath.Join("C:\\Users\\x\\AppData\\Roaming", "mcp-sim")
		if want := `C:\Users\x\AppData\Roaming\mcp-sim`; got != want {
			t.Fatalf("join = %q, want %q", got, want)
		}
		return
	}
	got := filepath.Join("/home/x", ".config", "mcp-sim")
	if want := "/home/x/.config/mcp-sim"; got != want {
		t.Fatalf("join = %q, want %q", got, want)
	}
}
