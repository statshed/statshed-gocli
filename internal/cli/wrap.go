package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	sserr "github.com/statshed/statshed-cli/internal/errors"
	"github.com/statshed/statshed-cli/internal/stream"
	"github.com/statshed/statshed-cli/internal/wrap"
)

func newWrapCmd(a *app) *cobra.Command {
	var (
		group, job     string
		minTime        float64
		swallow        bool
		regexPatterns  []string
		ignorePatterns []string
		ignoreCase     bool
		strict         bool
		reportExit     bool
		suppressExit   bool
		attachLog      bool
	)
	cmd := &cobra.Command{
		Use:   "wrap [flags] -- COMMAND [ARGS...]",
		Short: "Run a command, forward IO, and submit its output as progress updates",
		Long: `Run a command, forward IO, and submit its output as progress updates.

Each line the wrapped command writes to stdout or stderr is submitted as
a 'progress' status update to the given group/job, debounced the same way
as stream. Stdout is echoed to the wrapper's stdout and stderr to the
wrapper's stderr (unless --swallow). Stdin is forwarded to the child.

By default the wrapper exits with the wrapped command's exit code. With
--suppress-exitcode the wrapper always exits 0 on success.

Use -- to separate wrapper options from the wrapped command:

    statshed wrap -g misc -j current-time -- date
    statshed wrap -g ci -j build --report-exit -- make all`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if minTime < 0 {
				return fmt.Errorf("invalid value for --min-time: must be >= 0")
			}
			includes, err := stream.CompilePatterns(regexPatterns, ignoreCase)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid regex: %s\n", err)
				return exit(sserr.ExitInvalidArgs)
			}
			excludes, err := stream.CompilePatterns(ignorePatterns, ignoreCase)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid regex: %s\n", err)
				return exit(sserr.ExitInvalidArgs)
			}

			useStrict := strict || a.cfg.Submit.Strict
			// --attach-log is only meaningful alongside a final status submission.
			shouldReportExit := reportExit || attachLog

			sender := a.newProgressSender(group, job, useStrict)
			p := stream.New(minTime, includes, excludes, sender.send)

			// AIDEV-NOTE: Only open a log file when we might attach it. It is
			// always removed at the end regardless of submit success.
			var logFile *os.File
			var logPath string
			if attachLog {
				f, err := os.CreateTemp("", "statshed-wrap-*.log")
				if err != nil {
					fmt.Fprintf(os.Stderr, "statshed wrap: %s\n", err)
					return exit(sserr.ExitInvalidArgs)
				}
				logFile = f
				logPath = f.Name()
				defer func() {
					_ = logFile.Close()
					_ = os.Remove(logPath)
				}()
			}

			var logWriter io.Writer
			if logFile != nil {
				logWriter = logFile
			}

			res, runErr := wrap.Run(args, p, swallow, logWriter)
			if runErr != nil {
				var se *sserr.StatShedError
				if errors.As(runErr, &se) {
					// Strict-mode submission failure already reported by sender.
					return exit(sserr.ExitCode(sender.exitCode))
				}
				fmt.Fprintf(os.Stderr, "statshed wrap: %s\n", runErr)
				return exit(sserr.ExitInvalidArgs)
			}

			wrapperExit := int(sserr.ExitSuccess)

			if shouldReportExit {
				finalStatus := "success"
				if res.ExitCode != 0 {
					finalStatus = "error"
				}
				finalMessage := res.LastMessage
				if finalMessage == "" {
					finalMessage = fmt.Sprintf("exited with code %d", res.ExitCode)
				}
				attachPath := ""
				if attachLog && res.ExitCode != 0 && logFile != nil {
					_ = logFile.Sync()
					attachPath = logPath
				}
				if _, err := a.getClient().SubmitStatus(group, job, finalStatus, finalMessage, attachPath); err != nil {
					if useStrict {
						fmt.Fprintln(os.Stderr, a.formatter().Error(err.Error()))
						wrapperExit = int(sserr.ExitCodeOf(err))
					} else {
						logSubmitError(err, a.cfg.Submit)
						if !a.quiet && !a.cfg.Submit.Syslog {
							fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
						}
					}
				}
			}

			// Exit-code precedence: a wrapper-side strict failure wins, then
			// --suppress-exitcode, then the child's own exit code.
			if wrapperExit != int(sserr.ExitSuccess) {
				return exit(sserr.ExitCode(wrapperExit))
			}
			if suppressExit {
				return nil
			}
			if res.ExitCode != 0 {
				return &ExitError{Code: res.ExitCode}
			}
			return nil
		},
	}

	// AIDEV-NOTE: Disable interspersed flags so flags after the wrapped command
	// (e.g. `make --jobs`) are passed through as the command's own arguments.
	cmd.Flags().SetInterspersed(false)

	f := cmd.Flags()
	f.StringVarP(&group, "group", "g", "", "Group name")
	f.StringVarP(&job, "job", "j", "", "Job name")
	f.Float64Var(&minTime, "min-time", 60.0, "Minimum seconds between status submissions (debounced, last-wins)")
	f.BoolVar(&swallow, "swallow", false, "Do not echo the wrapped command's stdout/stderr through")
	f.StringArrayVar(&regexPatterns, "regex", nil, "Only submit lines matching one of these regexes (repeatable)")
	f.StringArrayVar(&ignorePatterns, "ignore", nil, "Skip lines matching one of these regexes (repeatable)")
	f.BoolVar(&ignoreCase, "ignore-case", false, "Case-insensitive regex matching for --regex and --ignore")
	f.BoolVar(&strict, "strict", false, "Exit with error code on submission failure (default: swallow errors)")
	f.BoolVar(&reportExit, "report-exit", false, "On exit, submit a final success/error status with the last line as the message")
	f.BoolVar(&suppressExit, "suppress-exitcode", false, "Do not propagate the wrapped command's exit code")
	f.BoolVar(&attachLog, "attach-log", false, "On non-zero exit, attach captured stdout+stderr as a log file (implies --report-exit)")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("job")
	_ = cmd.RegisterFlagCompletionFunc("group", a.completeGroupNames)
	_ = cmd.RegisterFlagCompletionFunc("job", a.completeJobNames)
	return cmd
}
