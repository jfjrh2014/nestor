package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jfjrh2014/nestor/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "nestor: %s\n", err)
		os.Exit(1)
	}
}
