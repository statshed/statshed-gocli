package cli

import (
	"github.com/spf13/cobra"
)

func newGroupsCmd(a *app) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List all groups with health summaries",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := a.getClient().GetGroups()
			if err != nil {
				return a.reportError(err)
			}
			a.output(a.resultFormatter(jsonOutput).Groups(data))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
