package config

import "testing"

func TestLoad(t *testing.T) {
	// t.Setenv restores the previous value when the test ends. Tests that mutate
	// process-wide environment variables must not run in parallel.
	t.Setenv("FAULTHARBOR_ADDR", "")
	t.Setenv("FAULTHARBOR_WEB_DIR", "")
	cfg, err := Load()
	if err != nil || cfg.Address != "127.0.0.1:8080" || cfg.WebDir != "web/dist" {
		t.Fatalf("unexpected defaults: %+v, %v", cfg, err)
	}
	t.Setenv("FAULTHARBOR_ADDR", "[::1]:9090")
	t.Setenv("FAULTHARBOR_WEB_DIR", "/app/web/dist")
	cfg, err = Load()
	if err != nil || cfg.Address != "[::1]:9090" || cfg.WebDir != "/app/web/dist" {
		t.Fatalf("unexpected overrides: %+v, %v", cfg, err)
	}
}

func TestRejectInvalidAddress(t *testing.T) {
	// Table-driven subtests apply the same expectation to several inputs while
	// keeping each failing case individually named in the test output.
	for _, address := range []string{"localhost", ":0", ":65536", ":http"} {
		t.Run(address, func(t *testing.T) {
			t.Setenv("FAULTHARBOR_ADDR", address)
			if _, err := Load(); err == nil {
				t.Fatalf("accepted invalid address %q", address)
			}
		})
	}
}
