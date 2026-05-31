package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	sserr "github.com/statshed/statshed-cli/internal/errors"
	"github.com/statshed/statshed-cli/internal/stream"
)

func newStreamCmd(a *app) *cobra.Command {
	var (
		group, job     string
		minTime        float64
		swallow        bool
		regexPatterns  []string
		ignorePatterns []string
		ignoreCase     bool
		strict         bool
	)
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream progress status updates from stdin",
		Long: `Stream progress status updates from stdin.

Each line of stdin is submitted as a "progress" status message to the
given group/job. Submissions are debounced: the first accepted line is
sent immediately, and further lines within --min-time are held (last
message wins) and flushed when the window elapses or on EOF.

--regex selects which lines are eligible for submission; --ignore
excludes them. Both use search semantics and are repeatable.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
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
			sender := a.newProgressSender(group, job, useStrict)
			p := stream.New(minTime, includes, excludes, sender.send)

			if err := runStreamLoop(os.Stdin, p, swallow); err != nil {
				// Strict-mode failure already reported by the sender.
				return exit(sserr.ExitCode(sender.exitCode))
			}
			if sender.exitCode != int(sserr.ExitSuccess) {
				return exit(sserr.ExitCode(sender.exitCode))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&group, "group", "g", "", "Group name")
	f.StringVarP(&job, "job", "j", "", "Job name")
	f.Float64Var(&minTime, "min-time", 60.0, "Minimum seconds between status submissions (debounced, last-wins)")
	f.BoolVar(&swallow, "swallow", false, "Do not echo stdin to stdout")
	f.StringArrayVar(&regexPatterns, "regex", nil, "Only submit lines matching one of these regexes (repeatable)")
	f.StringArrayVar(&ignorePatterns, "ignore", nil, "Skip lines matching one of these regexes (repeatable)")
	f.BoolVar(&ignoreCase, "ignore-case", false, "Case-insensitive regex matching for --regex and --ignore")
	f.BoolVar(&strict, "strict", false, "Exit with error code on submission failure (default: swallow errors)")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("job")
	_ = cmd.RegisterFlagCompletionFunc("group", a.completeGroupNames)
	_ = cmd.RegisterFlagCompletionFunc("job", a.completeJobNames)
	return cmd
}

// progressSender submits "progress" status updates, handling strict vs lenient
// error semantics and recording the resulting exit code.
type progressSender struct {
	a         *app
	group     string
	job       string
	useStrict bool
	exitCode  int
}

func (a *app) newProgressSender(group, job string, useStrict bool) *progressSender {
	return &progressSender{a: a, group: group, job: job, useStrict: useStrict}
}

// send submits one message. In strict mode it prints the error, records the
// exit code, and returns the error to break the IO loop. In lenient mode it
// logs/warns and returns nil so streaming continues.
func (s *progressSender) send(message string) error {
	_, err := s.a.getClient().SubmitStatus(s.group, s.job, "progress", message, "")
	if err == nil {
		return nil
	}
	if s.useStrict {
		fmt.Fprintln(os.Stderr, s.a.formatter().Error(err.Error()))
		s.exitCode = int(sserr.ExitCodeOf(err))
		return err
	}
	logSubmitError(err, s.a.cfg.Submit)
	if !s.a.quiet && !s.a.cfg.Submit.Syslog {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
	}
	return nil
}

// runStreamLoop drives the Processor from r, echoing each line (unless swallow)
// and flushing on the debounce timer, EOF, or SIGINT/SIGTERM.
func runStreamLoop(r io.Reader, p *stream.Processor, swallow bool) error {
	type lineEvent struct {
		text string
		eof  bool
	}
	events := make(chan lineEvent, 64)
	go func() {
		br := bufio.NewReader(r)
		for {
			line, err := br.ReadString('\n')
			if len(line) > 0 {
				events <- lineEvent{text: line}
			}
			if err != nil {
				events <- lineEvent{eof: true}
				return
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		var timerC <-chan time.Time
		var timer *time.Timer
		if d, ok := p.TimeUntilNextFlush(); ok {
			timer = time.NewTimer(d)
			timerC = timer.C
		}

		select {
		case ev := <-events:
			if timer != nil {
				timer.Stop()
			}
			if ev.eof {
				return p.FlushPending()
			}
			if !swallow {
				_, _ = io.WriteString(os.Stdout, ev.text)
			}
			if err := p.ProcessLine(ev.text); err != nil {
				return err
			}
		case <-timerC:
			if err := p.FlushIfDue(); err != nil {
				return err
			}
		case <-sigCh:
			// Best-effort flush on interrupt; ignore further errors.
			_ = p.FlushPending()
			return nil
		}
	}
}
