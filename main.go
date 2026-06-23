package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jfjrh2014/nestor/cmd"
)

// Build-time variables, set via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cmd.SetVersion(version, commit, date)

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "nestor: %s\n", err)
		os.Exit(1)
	}
}
