package cli

import (
	"github.com/spf13/cobra"

	"github.com/statshed/statshed-cli/internal/client"
	sserr "github.com/statshed/statshed-cli/internal/errors"
)

func newConfigCmd(a *app) *cobra.Command {
	var (
		progressTimeout  int
		stalenessTimeout int
		jsonOutput       bool
	)
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or update global configuration",
		Long: `View or update global configuration.

Without options, displays current configuration.
With options, updates the specified values.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var data client.JSON
			var err error

			pChanged := cmd.Flags().Changed("progress-timeout")
			sChanged := cmd.Flags().Changed("staleness-timeout")
			if pChanged || sChanged {
				data, err = a.getClient().UpdateConfig(
					intPtrIf(pChanged, progressTimeout),
					intPtrIf(sChanged, stalenessTimeout),
				)
			} else {
				data, err = a.getClient().GetConfig()
			}
			if err != nil {
				return a.reportError(err)
			}
			a.output(a.resultFormatter(jsonOutput).Config(data))
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVarP(&progressTimeout, "progress-timeout", "p", 0, "Progress timeout in minutes")
	f.IntVarP(&stalenessTimeout, "staleness-timeout", "s", 0, "Staleness timeout in hours")
	f.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newGroupConfigCmd(a *app) *cobra.Command {
	var (
		progressTimeout  int
		stalenessTimeout int
		resetProgress    bool
		resetStaleness   bool
		jsonOutput       bool
	)
	cmd := &cobra.Command{
		Use:   "group-config GROUP_NAME",
		Short: "View or update group-specific configuration",
		Long: `View or update group-specific configuration.

Without options, displays current group configuration.
With options, updates the specified values.
Use --reset-* options to revert to global defaults.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return a.completeGroupNames(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			groupName := args[0]
			pChanged := cmd.Flags().Changed("progress-timeout")
			sChanged := cmd.Flags().Changed("staleness-timeout")

			var data client.JSON
			var err error
			if pChanged || sChanged || resetProgress || resetStaleness {
				data, err = a.getClient().UpdateGroupConfig(
					groupName,
					intPtrIf(pChanged, progressTimeout),
					intPtrIf(sChanged, stalenessTimeout),
					resetProgress, resetStaleness,
				)
			} else {
				data, err = a.getClient().GetGroupConfig(groupName)
			}
			if err != nil {
				if sserr.IsNotFound(err) {
					return a.reportNotFound("Group '%s' not found", groupName)
				}
				return a.reportError(err)
			}
			a.output(a.resultFormatter(jsonOutput).GroupConfig(data))
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVarP(&progressTimeout, "progress-timeout", "p", 0, "Progress timeout override (minutes)")
	f.IntVarP(&stalenessTimeout, "staleness-timeout", "s", 0, "Staleness timeout override (hours)")
	f.BoolVar(&resetProgress, "reset-progress-timeout", false, "Reset to global default")
	f.BoolVar(&resetStaleness, "reset-staleness-timeout", false, "Reset to global default")
	f.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

// intPtrIf returns &v when changed is true, else nil.
func intPtrIf(changed bool, v int) *int {
	if changed {
		return &v
	}
	return nil
}
