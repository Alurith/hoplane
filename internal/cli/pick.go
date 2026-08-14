package cli

import "github.com/spf13/cobra"

func (s *commandState) newPickCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pick",
		Short: "Open the interactive connection picker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return s.pick(cmd.Context())
		},
	}
}
