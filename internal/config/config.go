package config

import "os"

type Config struct {
	Host    string
	Port    string
	Version string
}

func Load() Config {
	return Config{
		Host:    envOr("OPENMCP_HOST", "0.0.0.0"),
		Port:    envOr("OPENMCP_PORT", "8080"),
		Version: envOr("OPENMCP_VERSION", "dev"),
	}
}

func (c Config) Address() string { return c.Host + ":" + c.Port }

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
