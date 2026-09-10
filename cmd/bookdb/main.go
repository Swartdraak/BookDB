package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bookdb/bookdb/internal/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runtime.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
