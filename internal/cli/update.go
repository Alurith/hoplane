package cli

import (
	"fmt"

	"github.com/Alurith/hoplane/internal/selfupdate"
	"github.com/spf13/cobra"
)

func (s *commandState) newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update hoplane to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			update := s.dependencies.Update
			if update == nil {
				update = selfupdate.Update
			}
			result, err := update(cmd.Context(), s.version)
			if err != nil {
				return err
			}
			if result.Updated {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated hoplane from %s to %s\n", result.CurrentVersion, result.LatestVersion)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "hoplane is up to date (current %s, latest %s)\n", result.CurrentVersion, result.LatestVersion)
			return err
		},
	}
}
