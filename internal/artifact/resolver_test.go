package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRoots(t *testing.T) {
	roots := LoadRoots("/a:name=/b::/c")
	if len(roots) != 3 {
		t.Fatalf("got %d roots, want 3: %+v", len(roots), roots)
	}
	if roots[0].Name != "" || roots[0].Path != "/a" {
		t.Errorf("roots[0] = %+v, want anonymous /a", roots[0])
	}
	if roots[1].Name != "name" || roots[1].Path != "/b" {
		t.Errorf("roots[1] = %+v, want name=/b", roots[1])
	}
	if roots[2].Path != "/c" {
		t.Errorf("roots[2] = %+v, want /c", roots[2])
	}
	if LoadRoots("") != nil {
		t.Error("LoadRoots(\"\") should return nil")
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "build", "MyApp.app")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	roots := LoadRoots(dir + ":named=" + dir)

	tests := []struct {
		name string
		ref  string
		want string
	}{
		{"absolute", "/tmp/x/MyApp.app", "/tmp/x/MyApp.app"},
		{"relative under root", "build/MyApp.app", nested},
		{"artifact named root", "artifact://named/build/MyApp.app", nested},
		{"artifact anonymous", "artifact://build/MyApp.app", nested},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.ref, roots)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tt.ref, err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestResolveNotFound(t *testing.T) {
	_, err := Resolve("missing/thing.app", LoadRoots(t.TempDir()))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}

	// Unknown named root falls through to the not-found error.
	_, err = Resolve("artifact://unknown/thing.app", LoadRoots(t.TempDir()))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("named root miss: want ErrNotFound, got %v", err)
	}

	// No roots configured at all.
	_, err = Resolve("thing.app", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("no roots: want ErrNotFound, got %v", err)
	}
}
