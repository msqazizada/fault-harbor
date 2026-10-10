// Command faultharbor-collector holds Docker access and exposes sanitized inventory.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/msqazizada/fault-harbor/internal/dockerinspect"
	"github.com/msqazizada/fault-harbor/internal/inspection"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cfg dockerinspect.Config
	var socket string
	flag.StringVar(&cfg.Environment, "environment", "", "stable environment identifier (required)")
	flag.StringVar(&cfg.Instance, "source", "", "stable source identifier (required)")
	flag.StringVar(&cfg.Socket, "docker-socket", "/var/run/docker.sock", "Docker Engine Unix socket")
	flag.StringVar(&socket, "listen-socket", "/run/faultharbor/collector.sock", "collector Unix socket")
	flag.DurationVar(&cfg.Timeout, "timeout", 20*time.Second, "maximum collection duration")
	flag.DurationVar(&cfg.RequestTimeout, "request-timeout", 5*time.Second, "maximum duration per Docker request")
	flag.IntVar(&cfg.MaxContainers, "max-containers", 256, "maximum containers per snapshot (1–1000)")
	flag.IntVar(&cfg.MaxNetworks, "max-networks", 256, "maximum networks per snapshot (1–1000)")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected collector arguments")
	}
	if !filepath.IsAbs(socket) || socket == cfg.Socket {
		return errors.New("listen socket must be an absolute path distinct from the Docker socket")
	}
	source, err := dockerinspect.New(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	// The socket starts private, then opens to its configured group. Never remove
	// an existing socket: another collector may own it.
	syscall.Umask(0o077)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() { _ = listener.Close() }()
	// Group access allows the separate non-root consumer to connect.
	//nolint:gosec // G302: this Unix socket intentionally grants access to its configured group.
	if err = os.Chmod(socket, 0o660); err != nil {
		return fmt.Errorf("socket permissions: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{
		Handler:           inspection.NewHandler(source),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      cfg.Timeout + 5*time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(listener) }()
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = srv.Shutdown(shutdown); err != nil {
			_ = srv.Close()
			return err
		}
		return nil
	}
}
