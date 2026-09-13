package auth

import "testing"

func TestResolvePrecedence(t *testing.T) {
	tests := []struct {
		name     string
		envToken string
		cfgToken string
		want     string
		wantSrc  TokenSource
	}{
		{"env wins over config", "envtok", "cfgtok", "envtok", SourceEnv},
		{"config wins when no env", "", "cfgtok", "cfgtok", SourceConfig},
		{"generated when neither", "", "", "", SourceGenerated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, src := Resolve(tt.envToken, tt.cfgToken)
			if got != tt.want || src != tt.wantSrc {
				t.Errorf("Resolve(%q, %q) = (%q, %q), want (%q, %q)", tt.envToken, tt.cfgToken, got, src, tt.want, tt.wantSrc)
			}
		})
	}
}
