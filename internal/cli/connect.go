package cli

import (
	"context"
	"fmt"

	"github.com/Alurith/hoplane/internal/connector"
	"github.com/Alurith/hoplane/internal/domain"
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
	connection, err := s.findConnection(ctx, name)
	if err != nil {
		return err
	}

	return s.connectConnection(ctx, connection, connector.ConnectOptions{DryRun: dryRun})
}

func (s *commandState) connectConnection(
	ctx context.Context,
	connection domain.Connection,
	options connector.ConnectOptions,
) error {
	streams := connector.IO{
		Input:  s.dependencies.Input,
		Output: s.dependencies.Output,
		Errors: s.dependencies.Errors,
	}
	if err := s.registry.Connect(ctx, connection, streams, options); err != nil {
		return fmt.Errorf("connect %q: %w", connection.Name, err)
	}
	return nil
}
