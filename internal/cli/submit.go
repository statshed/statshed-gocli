package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var validStatuses = []string{"success", "error", "progress"}

func isValidStatus(s string) bool {
	for _, v := range validStatuses {
		if v == s {
			return true
		}
	}
	return false
}

func newSubmitCmd(a *app) *cobra.Command {
	var (
		group, job, status, message, logPath string
		strict                               bool
	)
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a job status update",
		Long: `Submit a job status update.

By default, errors are swallowed and the command exits with code 0.
Use --strict to exit with an error code on failure.

Optionally attach a log file with --log. If log uploads are disabled
on the server, the status update still succeeds but a warning is shown.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !isValidStatus(status) {
				return fmt.Errorf("invalid value for --status: %q (choose from success, error, progress)", status)
			}
			// click.Path(exists=True): a bad --log path is always a usage error.
			if logPath != "" {
				if info, err := os.Stat(logPath); err != nil || info.IsDir() {
					return fmt.Errorf("invalid value for --log: file %q does not exist", logPath)
				}
			}

			useStrict := strict || a.cfg.Submit.Strict

			data, err := a.getClient().SubmitStatus(group, job, status, message, logPath)
			if err != nil {
				if useStrict {
					return a.reportError(err)
				}
				// Lenient mode: log error but exit 0.
				logSubmitError(err, a.cfg.Submit)
				if !a.quiet && !a.cfg.Submit.Syslog {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
				}
				return nil
			}

			if !a.quiet {
				a.output(a.resultFormatter(false).SubmitSuccess(data))
				// AIDEV-NOTE: Backend returns a warning field when log uploads
				// are disabled but the status update still succeeded.
				if warning, _ := data["warning"].(string); warning != "" {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
				}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&group, "group", "g", "", "Group name")
	f.StringVarP(&job, "job", "j", "", "Job name")
	f.StringVarP(&status, "status", "s", "", "Status value (success, error, progress)")
	f.StringVarP(&message, "message", "m", "", "Optional status message")
	f.StringVarP(&logPath, "log", "l", "", "Path to log file to attach")
	f.BoolVar(&strict, "strict", false, "Exit with error code on failure (default: swallow errors)")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("job")
	_ = cmd.MarkFlagRequired("status")

	_ = cmd.RegisterFlagCompletionFunc("group", a.completeGroupNames)
	_ = cmd.RegisterFlagCompletionFunc("job", a.completeJobNames)
	_ = cmd.RegisterFlagCompletionFunc("status", completeStatusValues)
	return cmd
}
