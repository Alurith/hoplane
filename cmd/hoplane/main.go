package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Alurith/hoplane/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background(), os.Args[1:], cli.Dependencies{
		Input:  os.Stdin,
		Output: os.Stdout,
		Errors: os.Stderr,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "hoplane:", err)
		os.Exit(1)
	}
}
