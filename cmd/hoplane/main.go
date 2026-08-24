package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Alurith/hoplane/internal/askpass"
	"github.com/Alurith/hoplane/internal/cli"
	"github.com/Alurith/hoplane/internal/security"
	"github.com/Alurith/hoplane/internal/terminal"
)

var version = "dev"

func main() {
	if err := security.Harden(); err != nil {
		fmt.Fprintln(os.Stderr, "hoplane: harden process:", err)
		os.Exit(1)
	}
	if os.Getenv("HOPLANE_ASKPASS") == "1" {
		if err := askpass.Run(os.Args[1:]); err != nil {
			os.Exit(1)
		}
		return
	}

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
