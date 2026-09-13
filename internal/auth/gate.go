package auth

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
)

// ErrNonLoopbackNoAuth is returned when --insecure-no-auth is requested for
// a non-loopback listen address without explicit acknowledgment.
var ErrNonLoopbackNoAuth = errors.New("insecure-no-auth refused: listen address is not loopback; set --insecure-no-auth-ack or MCPSIM_TRUSTED_NETWORK=true to accept the risk")

// GateOptions carries the insecure-no-auth acknowledgment inputs.
type GateOptions struct {
	// Ack is true when --insecure-no-auth-ack was passed.
	Ack bool
	// TrustedEnv reports MCPSIM_TRUSTED_NETWORK=true.
	// If empty it is read from the environment.
	TrustedEnv string
}

// CheckInsecureNoAuth validates disabling auth for the given listen address.
// Loopback binds (127.x, ::1, localhost) are always allowed. Non-loopback
// binds (including ":9090" and "0.0.0.0", i.e. all interfaces) require an
// explicit acknowledgment via Ack or MCPSIM_TRUSTED_NETWORK=true.
func CheckInsecureNoAuth(listen string, opts GateOptions) error {
	trusted := opts.TrustedEnv
	if trusted == "" {
		trusted = os.Getenv("MCPSIM_TRUSTED_NETWORK")
	}
	if isTrusted(trusted) || opts.Ack {
		return nil
	}
	if IsLoopbackListen(listen) {
		return nil
	}
	return ErrNonLoopbackNoAuth
}

func isTrusted(v string) bool {
	b, err := strconv.ParseBool(v)
	return err == nil && b
}

// IsLoopbackListen reports whether the listen address binds only to a
// loopback interface. An empty or wildcard host (":9090", "0.0.0.0", "::")
// means all interfaces and is NOT loopback.
func IsLoopbackListen(listen string) bool {
	host := listen
	if hostPort, _, err := net.SplitHostPort(listen); err == nil {
		host = hostPort
	}
	host = strings.TrimSpace(host)
	switch host {
	case "":
		return false // all interfaces
	case "localhost":
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
