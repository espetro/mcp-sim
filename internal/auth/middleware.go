package auth

import (
	"context"
	"crypto/subtle"
	"net/http"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"time"
)

// staticTokenExpiration is set on verified static tokens: the SDK requires a
// non-zero expiration, and a local static token never expires in practice.
var staticTokenExpiration = time.Now().AddDate(100, 0, 0)

// Middleware returns HTTP middleware enforcing bearer auth with the given
// static local token. resourceMetadataURL, when non-empty, is advertised in
// the WWW-Authenticate challenge (RFC 9728 resource_metadata parameter).
// It is built on the SDK's RequireBearerToken so 401 responses carry
// spec-shaped challenges and TokenInfo is bound into the request context
// (UserID "local"), which the streamable transport uses to prevent
// cross-user session hijacking.
func Middleware(token, resourceMetadataURL string) func(http.Handler) http.Handler {
	verifier := sdkauth.TokenVerifier(func(ctx context.Context, tokenString string, r *http.Request) (*sdkauth.TokenInfo, error) {
		if subtle.ConstantTimeCompare([]byte(tokenString), []byte(token)) != 1 {
			return nil, sdkauth.ErrInvalidToken
		}
		return &sdkauth.TokenInfo{UserID: "local", Expiration: staticTokenExpiration}, nil
	})
	opts := &sdkauth.RequireBearerTokenOptions{}
	if resourceMetadataURL != "" {
		opts.ResourceMetadataURL = resourceMetadataURL
	}
	return sdkauth.RequireBearerToken(verifier, opts)
}
