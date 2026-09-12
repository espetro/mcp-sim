package auth

import (
	"errors"
	"testing"
)

func TestCheckInsecureNoAuthMatrix(t *testing.T) {
	tests := []struct {
		name        string
		listen      string
		ack         bool
		trustedEnv  string
		wantRefused bool
	}{
		{"loopback ok without ack", "127.0.0.1:9090", false, "", false},
		{"localhost ok without ack", "localhost:9090", false, "", false},
		{"ipv6 loopback ok", "[::1]:9090", false, "", false},
		{"all-interfaces :9090 refused", ":9090", false, "", true},
		{"0.0.0.0 refused", "0.0.0.0:9090", false, "", true},
		{"all-interfaces with ack ok", ":9090", true, "", false},
		{"all-interfaces with trusted env ok", ":9090", false, "true", false},
		{"trusted env false still refused", ":9090", false, "false", true},
		{"trusted env garbage still refused", ":9090", false, "yes", true},
		{"non-loopback LAN addr refused", "192.168.1.10:9090", false, "", true},
		{"non-loopback with ack ok", "192.168.1.10:9090", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckInsecureNoAuth(tt.listen, GateOptions{Ack: tt.ack, TrustedEnv: tt.trustedEnv})
			if tt.wantRefused && !errors.Is(err, ErrNonLoopbackNoAuth) {
				t.Errorf("CheckInsecureNoAuth(%q) = %v, want ErrNonLoopbackNoAuth", tt.listen, err)
			}
			if !tt.wantRefused && err != nil {
				t.Errorf("CheckInsecureNoAuth(%q) = %v, want nil", tt.listen, err)
			}
		})
	}
}

func TestIsLoopbackListen(t *testing.T) {
	tests := []struct {
		listen string
		want   bool
	}{
		{"127.0.0.1:9090", true},
		{"127.9.9.9:1", true},
		{"localhost:9090", true},
		{"[::1]:9090", true},
		{":9090", false},
		{"0.0.0.0:9090", false},
		{"192.168.1.10:9090", false},
		{"::", false},
	}
	for _, tt := range tests {
		if got := IsLoopbackListen(tt.listen); got != tt.want {
			t.Errorf("IsLoopbackListen(%q) = %v, want %v", tt.listen, got, tt.want)
		}
	}
}
