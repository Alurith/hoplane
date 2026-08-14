package cli

import (
	"github.com/spf13/cobra"

	"github.com/Alurith/hoplane/internal/output"
)

func (s *commandState) newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured connections as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalog, err := s.loadCatalog(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteList(s.dependencies.Output, catalog)
		},
	}
}
