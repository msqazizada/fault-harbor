// Command faultharbor runs the HTTP API and web interface.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/msqazizada/fault-harbor/internal/config"
	"github.com/msqazizada/fault-harbor/internal/server"
)

// The build sets this variable with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		code := inspect(ctx, os.Args[1:], os.Stdout, os.Stderr)
		stop()
		os.Exit(code)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run owns startup and cleanup. Returning errors to main lets deferred cleanup
// finish before os.Exit, which would otherwise skip deferred calls.
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// defer schedules cleanup when run returns, including on error paths.
	defer stop()

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           server.NewHandler(version, os.DirFS(cfg.WebDir)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// ListenAndServe blocks, so a goroutine runs it concurrently. The buffered
	// channel lets it report one result even if shutdown is already in progress.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "address", cfg.Address, "version", version)
		serverErr <- srv.ListenAndServe()
	}()

	// select waits for either a listener result or cancellation from a signal.
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		// Restore signal handling so a second interrupt can terminate immediately.
		stop()
		logger.Info("shutting down")
		// The signal context is already cancelled. A fresh context gives active
		// requests a separate grace period to finish before connections close.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
