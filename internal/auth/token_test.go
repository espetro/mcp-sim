package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateUniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		token, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if len(token) == 0 {
			t.Fatal("Generate() returned empty token")
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("Generate() returned duplicate token %q at iteration %d", token, i)
		}
		seen[token] = struct{}{}
	}
}

func TestLoadOrCreateCreatesWithPerm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "token")
	token, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error: %v", err)
	}
	if token == "" {
		t.Fatal("LoadOrCreate() returned empty token")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("token file perms = %o, want 600", got)
	}
}

func TestLoadOrCreateReusesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("first LoadOrCreate() error: %v", err)
	}
	// Restart simulation: the token must never be rotated.
	second, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate() error: %v", err)
	}
	if first != second {
		t.Errorf("token rotated on restart: %q != %q", first, second)
	}
}

func TestLoadOrCreateEmptyFileRegenerates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error: %v", err)
	}
	if token == "" {
		t.Fatal("LoadOrCreate() returned empty token for empty file")
	}
}

func TestWriteSnippet(t *testing.T) {
	dir := t.TempDir()
	snippetPath := filepath.Join(dir, "sub", "client-snippet.json")
	if err := WriteSnippet(snippetPath, "http://localhost:9090/mcp", "tok123"); err != nil {
		t.Fatalf("WriteSnippet() error: %v", err)
	}
	data, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	for _, want := range []string{`"url": "http://localhost:9090/mcp"`, `Bearer tok123`, `"mcpServers"`} {
		if !contains(string(data), want) {
			t.Errorf("snippet missing %s; got:\n%s", want, data)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
