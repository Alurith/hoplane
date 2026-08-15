package cli

import (
	"github.com/spf13/cobra"

	"github.com/Alurith/hoplane/internal/output"
)

func (s *commandState) newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <connection>",
		Short: "Show one normalized connection as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := s.findConnection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return output.WriteConnection(s.dependencies.Output, connection)
		},
	}
}
