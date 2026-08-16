package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Alurith/hoplane/internal/cli"
	"github.com/Alurith/hoplane/internal/terminal"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx, os.Args[1:], cli.Dependencies{
		Input:   os.Stdin,
		Output:  os.Stdout,
		Errors:  os.Stderr,
		Version: version,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "hoplane:", terminal.EscapeControls(err.Error()))
		os.Exit(1)
	}
}
