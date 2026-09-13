package auth

import (
	"encoding/json"
	"net/http"
)

// MetadataPath is the RFC 9728 well-known protected resource metadata path.
const MetadataPath = "/.well-known/oauth-protected-resource"

// metadata is the served subset of RFC 9728 Protected Resource Metadata.
// resourceURI may be empty (OmitResource) when the external URL is unknown,
// e.g. behind a proxy or Tailscale Serve; clients then rely on the
// WWW-Authenticate challenge alone.
type metadata struct {
	Resource               string   `json:"resource,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

// MetadataHandler serves the RFC 9728 document unauthenticated at
// MetadataPath. It advertises header bearer auth only, and never advertises
// authorization servers: mcp-sim is a static-token resource, never an
// OAuth authorization server.
func MetadataHandler(resourceURL string) http.Handler {
	m := metadata{
		Resource:               resourceURL,
		BearerMethodsSupported: []string{"header"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m)
	})
}
