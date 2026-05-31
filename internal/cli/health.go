package cli

import (
	"github.com/spf13/cobra"

	sserr "github.com/statshed/statshed-cli/internal/errors"
)

func newHealthCmd(a *app) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check overall system health",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := a.getClient().GetHealth()
			if err != nil {
				return a.reportError(err)
			}
			a.output(a.resultFormatter(jsonOutput).Health(data))

			// Exit with code 1 if unhealthy.
			if status, _ := data["status"].(string); status == "unhealthy" {
				return exit(sserr.ExitUnhealthy)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
