// Package config reads and validates settings before the server starts.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// Config holds validated startup settings. Exported names start with a capital
// letter in Go; helpers such as envOrDefault remain private to this package.
type Config struct {
	Address string
	WebDir  string
}

// Load applies environment overrides to defaults and rejects invalid settings.
// Callers must check the error before using the returned configuration.
func Load() (Config, error) {
	cfg := Config{
		Address: envOrDefault("FAULTHARBOR_ADDR", "127.0.0.1:8080"),
		WebDir:  envOrDefault("FAULTHARBOR_WEB_DIR", "web/dist"),
	}
	// := declares variables with inferred types; _ discards the host result.
	// SplitHostPort handles bracketed IPv6 addresses as well as hostnames.
	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		// Config{} is the zero value. %w wraps the original error so callers can
		// still inspect its cause with errors.Is or errors.As.
		return Config{}, fmt.Errorf("FAULTHARBOR_ADDR must be host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("FAULTHARBOR_ADDR port must be between 1 and 65535")
	}
	return cfg, nil // A nil error indicates success.
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
