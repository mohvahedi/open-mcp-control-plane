package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
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
}

func Load() Config {
	return Config{
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
	}
}

func (c Config) Address() string { return c.Host + ":" + c.Port }

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
