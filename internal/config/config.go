package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/security"
)

type Config struct {
	Host                string
	Port                string
	Version             string
	DatabasePath        string
	AdminBootstrapToken string
	RequestTimeout      time.Duration
	RegistryBaseURL     string
	RegistryTimeout     time.Duration
	RegistryPageLimit   int
	UseFakeRuntime      bool
	DockerBinary        string
	GatewayBindPath     string
	SecretsMasterKey    string
	SecretsBackend      string // local | env | file
	SecretsFileDir      string

	// OIDC admin identity (optional).
	OIDCEnabled         bool
	OIDCIssuerURL       string
	OIDCClientID        string
	OIDCClientSecret    string
	OIDCRedirectURL     string
	OIDCScopes          []string
	OIDCAllowedEmails   []string
	OIDCAllowedSubjects []string
	OIDCSessionTTL      time.Duration
	// SessionHMACSecret signs post-login admin session cookies/tokens.
	// Defaults to SecretsMasterKey when empty.
	SessionHMACSecret string

	// MCP session TTL for Streamable HTTP / SSE sessions.
	MCPSessionTTL time.Duration
}

func Load() Config {
	c := Config{
		Host:                envOr("OPENMCP_HOST", "0.0.0.0"),
		Port:                envOr("OPENMCP_PORT", "8080"),
		Version:             envOr("OPENMCP_VERSION", "dev"),
		DatabasePath:        envOr("OPENMCP_DB_PATH", "file:openmcp.db?_pragma=busy_timeout(5000)"),
		AdminBootstrapToken: os.Getenv("OPENMCP_BOOTSTRAP_ADMIN_TOKEN"),
		RequestTimeout:      envDurationOr("OPENMCP_REQUEST_TIMEOUT", 10*time.Second),
		RegistryBaseURL:     envOr("OPENMCP_REGISTRY_BASE_URL", "https://registry.modelcontextprotocol.io"),
		RegistryTimeout:     envDurationOr("OPENMCP_REGISTRY_TIMEOUT", 5*time.Second),
		RegistryPageLimit:   envIntOr("OPENMCP_REGISTRY_PAGE_LIMIT", 25),
		UseFakeRuntime:      envBoolOr("OPENMCP_USE_FAKE_RUNTIME", false),
		DockerBinary:        envOr("OPENMCP_DOCKER_BINARY", "docker"),
		GatewayBindPath:     envOr("OPENMCP_GATEWAY_PATH", "/gateway"),
		SecretsMasterKey:    envOr("OPENMCP_SECRETS_MASTER_KEY", "dev-only-change-me"),
		SecretsBackend:      envOr("OPENMCP_SECRETS_BACKEND", "local"),
		SecretsFileDir:      envOr("OPENMCP_SECRETS_FILE_DIR", "./secrets-data"),

		OIDCEnabled:         envBoolOr("OPENMCP_OIDC_ENABLED", false),
		OIDCIssuerURL:       os.Getenv("OPENMCP_OIDC_ISSUER_URL"),
		OIDCClientID:        os.Getenv("OPENMCP_OIDC_CLIENT_ID"),
		OIDCClientSecret:    os.Getenv("OPENMCP_OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:     envOr("OPENMCP_OIDC_REDIRECT_URL", "http://127.0.0.1:8080/v1/auth/oidc/callback"),
		OIDCScopes:          security.ParseAllowedCSV(envOr("OPENMCP_OIDC_SCOPES", "openid,profile,email")),
		OIDCAllowedEmails:   security.ParseAllowedCSV(os.Getenv("OPENMCP_OIDC_ALLOWED_EMAILS")),
		OIDCAllowedSubjects: security.ParseAllowedCSV(os.Getenv("OPENMCP_OIDC_ALLOWED_SUBJECTS")),
		OIDCSessionTTL:      envDurationOr("OPENMCP_OIDC_SESSION_TTL", 8*time.Hour),
		SessionHMACSecret:   os.Getenv("OPENMCP_SESSION_HMAC_SECRET"),

		MCPSessionTTL: envDurationOr("OPENMCP_MCP_SESSION_TTL", 30*time.Minute),
	}
	if c.SessionHMACSecret == "" {
		c.SessionHMACSecret = c.SecretsMasterKey
	}
	return c
}

func (c Config) Address() string { return c.Host + ":" + c.Port }

func (c Config) OIDC() security.OIDCConfig {
	return security.OIDCConfig{
		Enabled:         c.OIDCEnabled,
		IssuerURL:       c.OIDCIssuerURL,
		ClientID:        c.OIDCClientID,
		ClientSecret:    c.OIDCClientSecret,
		RedirectURL:     c.OIDCRedirectURL,
		Scopes:          c.OIDCScopes,
		AllowedEmails:   c.OIDCAllowedEmails,
		AllowedSubjects: c.OIDCAllowedSubjects,
	}
}

func (c Config) Validate() error {
	if c.Host == "" || c.Port == "" {
		return errors.New("host and port are required")
	}
	if _, err := strconv.Atoi(c.Port); err != nil {
		return fmt.Errorf("port must be numeric: %w", err)
	}
	if c.DatabasePath == "" {
		return errors.New("database path is required")
	}
	if c.RequestTimeout <= 0 || c.RegistryTimeout <= 0 {
		return errors.New("timeouts must be positive")
	}
	if c.RegistryPageLimit <= 0 || c.RegistryPageLimit > 100 {
		return errors.New("registry page limit must be between 1 and 100")
	}
	if c.SecretsMasterKey == "" {
		return errors.New("secrets master key is required")
	}
	switch strings.ToLower(c.SecretsBackend) {
	case "local", "aes", "aes-gcm", "env", "file", "":
	default:
		return fmt.Errorf("invalid secrets backend %q", c.SecretsBackend)
	}
	if c.OIDCEnabled {
		if c.OIDCIssuerURL == "" || c.OIDCClientID == "" || c.OIDCRedirectURL == "" {
			return errors.New("oidc issuer, client id, and redirect url required when OIDC enabled")
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}

func envBoolOr(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return fallback
}
