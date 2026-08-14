package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Alurith/hoplane/internal/output"
)

func (s *commandState) newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <connection>",
		Short: "Show one normalized connection as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := s.loadCatalog(cmd.Context())
			if err != nil {
				return err
			}
			connection, ok := catalog.Find(args[0])
			if !ok {
				return fmt.Errorf("connection %q not found", args[0])
			}
			return output.WriteConnection(s.dependencies.Output, connection)
		},
	}
}
