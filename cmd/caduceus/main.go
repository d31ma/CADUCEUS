// Command caduceus runs the CADUCEUS production service and operator CLI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
	"github.com/d31ma/caduceus/internal/platform/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, buildinfo.Current()))
}
