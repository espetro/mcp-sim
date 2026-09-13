package auth

// TokenSource describes where a resolved token came from.
type TokenSource string

// Token sources in resolution precedence order.
const (
	SourceEnv       TokenSource = "env"       // MCPSIM_AUTH_TOKEN
	SourceConfig    TokenSource = "config"    // server.auth.token in config.yaml
	SourceGenerated TokenSource = "generated" // first-boot auto-generation
)

// Resolve picks the token by precedence: env token > config file token >
// generated. The caller passes the env value (MCPSIM_AUTH_TOKEN) and the
// config file value (server.auth.token) explicitly so tests can inject them.
func Resolve(envToken, cfgToken string) (string, TokenSource) {
	if envToken != "" {
		return envToken, SourceEnv
	}
	if cfgToken != "" {
		return cfgToken, SourceConfig
	}
	return "", SourceGenerated
}
