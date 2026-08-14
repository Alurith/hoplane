package cli

import (
	"context"
	"fmt"

	"github.com/Alurith/hoplane/internal/connector"
	"github.com/spf13/cobra"
)

func (s *commandState) newConnectCommand() *cobra.Command {
	var dryRun bool

	command := &cobra.Command{
		Use:   "connect <connection>",
		Short: "Connect to a named connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.connect(cmd.Context(), args[0], dryRun)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show the command without executing it")
	return command
}

func (s *commandState) connect(ctx context.Context, name string, dryRun bool) error {
	catalog, err := s.loadCatalog(ctx)
	if err != nil {
		return err
	}

	connection, ok := catalog.Find(name)
	if !ok {
		return fmt.Errorf("connection %q not found", name)
	}

	streams := connector.IO{
		Input:  s.dependencies.Input,
		Output: s.dependencies.Output,
		Errors: s.dependencies.Errors,
	}
	if err := s.registry.Connect(
		ctx,
		connection,
		streams,
		connector.ConnectOptions{DryRun: dryRun},
	); err != nil {
		return fmt.Errorf("connect %q: %w", connection.Name, err)
	}
	return nil
}
