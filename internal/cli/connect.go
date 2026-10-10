package cli

import (
	"context"
	"fmt"

	"github.com/Alurith/hoplane/internal/connector"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/charmbracelet/x/ansi"
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

	return s.connectConnection(ctx, connection, dryRun)
}

func (s *commandState) connectConnection(
	ctx context.Context,
	connection domain.Connection,
	dryRun bool,
) error {
	streams := connector.IO{
		Input:  s.dependencies.Input,
		Output: s.dependencies.Output,
		Errors: s.dependencies.Errors,
	}
	if !dryRun {
		s.reportProgramStatus(ansi.ProgramStateWorking, connection.Name)
	}

	err := s.registry.Connect(ctx, connection, streams, dryRun)
	if !dryRun {
		state := ansi.ProgramStateDone
		if err != nil {
			state = ansi.ProgramStateError
		}
		s.reportProgramStatus(state, connection.Name)
	}
	if err != nil {
		return fmt.Errorf("connect %q: %w", connection.Name, err)
	}
	return nil
}
