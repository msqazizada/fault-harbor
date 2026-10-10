package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/msqazizada/fault-harbor/internal/inspection"
)

// inspect returns an exit status after cleanup. Partial JSON remains useful to
// callers, while status 2 prevents automation from assuming a complete inventory.
func inspect(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) < 2 || args[0] != "inspect" || args[1] != "docker" {
		_, _ = fmt.Fprintln(errOut, "usage: faultharbor inspect docker [--socket /run/faultharbor/collector.sock]")
		return 1
	}
	flags := flag.NewFlagSet("inspect docker", flag.ContinueOnError)
	flags.SetOutput(errOut)
	socket := flags.String("socket", "/run/faultharbor/collector.sock", "collector Unix socket")
	if err := flags.Parse(args[2:]); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(errOut, "unexpected inspection arguments")
		return 1
	}
	ctx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	snap, err := inspection.Fetch(ctx, *socket)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err = enc.Encode(snap); err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	if snap.State == "partial" {
		return 2
	}
	return 0
}
