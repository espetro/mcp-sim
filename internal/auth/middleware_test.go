package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, token, reqToken string) *httptest.ResponseRecorder {
	t.Helper()
	handler := Middleware(token, "http://localhost/mcp")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	if reqToken != "" {
		req.Header.Set("Authorization", "Bearer "+reqToken)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		reqToken   string
		wantStatus int
	}{
		{"missing token", "secret", "", http.StatusUnauthorized},
		{"bad token", "secret", "wrong", http.StatusUnauthorized},
		{"valid token", "secret", "secret", http.StatusOK},
		{"empty expected token rejects everything", "", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, tt.token, tt.reqToken)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestMiddlewareChallengeHeader(t *testing.T) {
	rec := serve(t, "secret", "")
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("missing WWW-Authenticate header on 401")
	} else if !contains(got, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want Bearer challenge", got)
	}
	// Success path must not carry a challenge.
	rec = serve(t, "secret", "secret")
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("WWW-Authenticate on 200 = %q, want empty", got)
	}
}
