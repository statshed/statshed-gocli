package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// AIDEV-NOTE: Dynamic completion queries the API silently and falls back to no
// suggestions on any error, so completion never breaks when the server is down.
// cobra ships the `completion` subcommand and the __complete machinery; these
// functions back the --group/--job/--status flag completions.

func (a *app) completeGroupNames(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if a.cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	data, err := a.getClient().GetGroups()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	groups, _ := data["groups"].([]any)
	var out []string
	for _, g := range groups {
		group, _ := g.(map[string]any)
		name, _ := group["name"].(string)
		if name != "" && strings.HasPrefix(name, toComplete) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func (a *app) completeJobNames(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if a.cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	group, _ := cmd.Flags().GetString("group")
	if group == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	data, err := a.getClient().GetJobs(group)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	jobs, _ := data["jobs"].([]any)
	var out []string
	for _, j := range jobs {
		job, _ := j.(map[string]any)
		name, _ := job["name"].(string)
		if name != "" && strings.HasPrefix(name, toComplete) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeStatusValues(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	all := []string{
		"success\tJob completed successfully",
		"error\tJob encountered an error",
		"progress\tJob is currently running",
	}
	var out []string
	for _, s := range all {
		if strings.HasPrefix(s, toComplete) {
			out = append(out, s)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
