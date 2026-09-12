// Package auth implements local bearer-token authentication for the HTTP
// surface of mcp-sim: token generation and persistence, resolution
// precedence, bearer middleware, RFC 9728 metadata, and the
// insecure-no-auth gate. stdio transport stays auth free.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

// tokenBytes is the entropy size of a generated token: 32 bytes of
// crypto/rand, base64url encoded without padding.
const tokenBytes = 32

// tokenPerm is the file mode for the persisted token file. The token is the
// only secret guarding the server, so it must be owner read/write only.
const tokenPerm = 0o600

// Generate returns a fresh random token: 32 bytes from crypto/rand,
// base64url encoded without padding.
func Generate() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// LoadOrCreate returns the token stored at path, creating it with a newly
// generated token if missing. An existing token is reused as-is: the server
// never rotates the token on restart, so clients keep working across
// reboots. The file is always written with 0600 permissions.
func LoadOrCreate(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		token := string(data)
		if token != "" {
			return token, nil
		}
		// Empty file: fall through and (re)create.
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading token file %s: %w", path, err)
	}

	token, err := Generate()
	if err != nil {
		return "", err
	}
	if err := WriteFile(path, token); err != nil {
		return "", err
	}
	return token, nil
}

// WriteFile persists token at path with 0600 permissions, creating parent
// directories as needed (0755 for the dirs; the file itself stays 0600).
func WriteFile(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating token dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), tokenPerm); err != nil {
		return fmt.Errorf("writing token file %s: %w", path, err)
	}
	return nil
}

// WriteSnippet writes a ready-to-paste client snippet (Claude/Cursor
// mcpServers JSON shape) at snippetPath pointing at url with the bearer
// token inline. Written 0600 since it embeds the token.
func WriteSnippet(snippetPath, url, token string) error {
	snippet := fmt.Sprintf(`{
  "mcpServers": {
    "mcp-sim": {
      "url": %q,
      "headers": {
        "Authorization": "Bearer %s"
      }
    }
  }
}
`, url, token)

	if err := os.MkdirAll(filepath.Dir(snippetPath), 0o750); err != nil {
		return fmt.Errorf("creating snippet dir: %w", err)
	}
	if err := os.WriteFile(snippetPath, []byte(snippet), tokenPerm); err != nil {
		return fmt.Errorf("writing snippet %s: %w", snippetPath, err)
	}
	return nil
}
