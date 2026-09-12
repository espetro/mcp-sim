package auth

import (
	"context"
	"crypto/subtle"
	"net/http"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// Middleware returns HTTP middleware enforcing bearer auth with the given
// static local token. It is built on the SDK's RequireBearerToken so 401
// responses carry spec-shaped WWW-Authenticate challenges and TokenInfo is
// bound into the request context (UserID "local"), which the streamable
// transport uses to prevent cross-user session hijacking.
func Middleware(token string) func(http.Handler) http.Handler {
	verifier := sdkauth.TokenVerifier(func(ctx context.Context, tokenString string, r *http.Request) (*sdkauth.TokenInfo, error) {
		if subtle.ConstantTimeCompare([]byte(tokenString), []byte(token)) != 1 {
			return nil, sdkauth.ErrInvalidToken
		}
		return &sdkauth.TokenInfo{UserID: "local"}, nil
	})
	return sdkauth.RequireBearerToken(verifier, nil)
}
