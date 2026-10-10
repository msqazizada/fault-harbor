package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

type Config struct {
	Address string
	WebDir  string
}

func Load() (Config, error) {
	cfg := Config{
		Address: envOrDefault("FAULTHARBOR_ADDR", "127.0.0.1:8080"),
		WebDir:  envOrDefault("FAULTHARBOR_WEB_DIR", "web/dist"),
	}
	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return Config{}, fmt.Errorf("FAULTHARBOR_ADDR must be host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("FAULTHARBOR_ADDR port must be between 1 and 65535")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
