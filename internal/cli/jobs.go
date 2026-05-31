package cli

import (
	"github.com/spf13/cobra"

	sserr "github.com/statshed/statshed-cli/internal/errors"
)

func newJobsCmd(a *app) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "jobs GROUP_NAME",
		Short: "List all jobs in a group",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return a.completeGroupNames(cmd, args, toComplete)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			groupName := args[0]
			data, err := a.getClient().GetJobs(groupName)
			if err != nil {
				if sserr.IsNotFound(err) {
					return a.reportNotFound("Group '%s' not found", groupName)
				}
				return a.reportError(err)
			}
			a.output(a.resultFormatter(jsonOutput).Jobs(data))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
