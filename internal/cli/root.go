// Package cli wires the StatShed commands together with cobra.
//
// AIDEV-NOTE: Global options live on the root command's persistent flags and
// are resolved into a *app in PersistentPreRunE, which every subcommand reads.
// Commands print their own formatted errors and return an *ExitError carrying
// the desired process exit code; Execute maps that to the return value.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/statshed/statshed-cli/internal/client"
	"github.com/statshed/statshed-cli/internal/config"
	sserr "github.com/statshed/statshed-cli/internal/errors"
	"github.com/statshed/statshed-cli/internal/output"
)

// ExitError carries a process exit code up to Execute.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }

// exit returns an *ExitError for the given code.
func exit(code sserr.ExitCode) *ExitError { return &ExitError{Code: int(code)} }

// app holds resolved global flags and lazily-built shared objects.
type app struct {
	// global flag values
	url        string
	configPath string
	quiet      bool
	noColor    bool
	jsonOutput bool

	cfg    *config.Config
	client *client.Client
}

// getClient lazily builds the API client from config.
func (a *app) getClient() *client.Client {
	if a.client == nil {
		a.client = client.New(a.cfg.URL, a.cfg.Timeout, a.cfg.Retries, a.cfg.RetryDelay)
	}
	return a.client
}

// formatter returns the configured output formatter.
func (a *app) formatter() output.Formatter {
	return output.New(a.cfg.OutputFormat, a.cfg.Color)
}

// resultFormatter returns a JSON formatter when JSON output is requested
// (per-command flag or the global --json), else the configured formatter.
func (a *app) resultFormatter(jsonFlag bool) output.Formatter {
	if jsonFlag || a.jsonOutput {
		return output.JSONFormatter{}
	}
	return a.formatter()
}

// output prints text to stdout unless in quiet mode.
func (a *app) output(text string) {
	if !a.quiet {
		fmt.Println(text)
	}
}

// reportError prints a formatted error to stderr and returns the exit error.
func (a *app) reportError(err error) *ExitError {
	fmt.Fprintln(os.Stderr, a.formatter().Error(err.Error()))
	return &ExitError{Code: int(sserr.ExitCodeOf(err))}
}

// reportNotFound prints a formatted not-found message and returns exit 11.
func (a *app) reportNotFound(format string, args ...any) *ExitError {
	fmt.Fprintln(os.Stderr, a.formatter().Error(fmt.Sprintf(format, args...)))
	return exit(sserr.ExitNotFound)
}

// Execute builds and runs the root command, returning the process exit code.
func Execute(version string) int {
	a := &app{}
	root := newRootCmd(a, version)
	root.SilenceUsage = true
	root.SilenceErrors = true

	err := root.Execute()
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	// cobra argument/usage errors (unknown flag, bad value, missing required).
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	return int(sserr.ExitInvalidArgs)
}

func newRootCmd(a *app, version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "statshed",
		Short:         "Command-line interface for StatShed status dashboard",
		Long:          "StatShed CLI - Command-line interface for StatShed status dashboard.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// AIDEV-NOTE: Resolve global config before any subcommand runs. A bad
		// --config path (or invalid file) fails here with exit code 5, matching
		// the Python CLI's "Configuration error:" behavior.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.FromSources(a.configPath, a.url, a.noColor, a.jsonOutput)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
				return exit(sserr.ExitConfig)
			}
			a.cfg = cfg
			return nil
		},
	}

	root.SetVersionTemplate("statshed version {{.Version}}\n")

	flags := root.PersistentFlags()
	flags.StringVarP(&a.url, "url", "u", "", "StatShed API URL")
	flags.StringVarP(&a.configPath, "config", "c", "", "Path to config file")
	flags.BoolVarP(&a.quiet, "quiet", "q", false, "Suppress non-error output")
	flags.BoolVar(&a.noColor, "no-color", false, "Disable colored output")
	flags.BoolVar(&a.jsonOutput, "json", false, "Output in JSON format")

	root.AddCommand(
		newSubmitCmd(a),
		newHealthCmd(a),
		newStreamCmd(a),
		newWrapCmd(a),
		newGroupsCmd(a),
		newJobsCmd(a),
		newConfigCmd(a),
		newGroupConfigCmd(a),
	)
	return root
}
