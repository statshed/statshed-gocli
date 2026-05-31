// Package wrap owns the `wrap` command's subprocess plumbing: spawning the
// child, forwarding stdin, multiplexing stdout/stderr through a stream.Processor,
// and propagating signals.
//
// AIDEV-NOTE: Behavior mirrors the Python wrap module.
//   - Stdin from the wrapper is piped to the child; EOF on our stdin closes the
//     child's stdin so commands that read to EOF (cat, sort) can finish.
//   - Child stdout and stderr are read concurrently; each complete line is fed
//     into the Processor and (unless swallow) echoed to our stdout/stderr.
//   - If logFile is set, line bytes (both streams, in arrival order) are tee'd
//     into it for later attachment to a final status.
//   - SIGINT/SIGTERM received by the wrapper are forwarded to the child.
package wrap

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/statshed/statshed-cli/internal/stream"
)

// Result is the outcome of a wrapped run.
type Result struct {
	// ExitCode is the shell-style exit code (128+signum for signal deaths).
	ExitCode int
	// LastMessage is the most recently submitted message, useful as the body
	// of a final status update. Empty if nothing was submitted.
	LastMessage string
}

type lineEvent struct {
	streamName string
	text       string
	eof        bool
}

// normalizeExitCode converts a process state to a shell-style exit code:
// signal-terminated processes report 128 + signal number.
func normalizeExitCode(state *os.ProcessState) int {
	if state == nil {
		return 0
	}
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return state.ExitCode()
}

// Run spawns argv and drives IO until it exits. It returns the result and any
// error from the Processor's send function (strict-mode submission failure).
func Run(argv []string, p *stream.Processor, swallow bool, logFile io.Writer) (Result, error) {
	cmd := exec.Command(argv[0], argv[1:]...)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, err
	}

	if err := cmd.Start(); err != nil {
		return Result{}, err
	}

	// Capture the last submitted message by wrapping the send function.
	original := p.Send()
	var lastMessage string
	p.SetSend(func(msg string) error {
		lastMessage = msg
		return original(msg)
	})
	defer p.SetSend(original)

	// Forward SIGINT/SIGTERM to the child for the duration of the run.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	stopSig := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigCh:
				if cmd.Process != nil {
					_ = cmd.Process.Signal(s)
				}
			case <-stopSig:
				return
			}
		}
	}()
	defer func() {
		signal.Stop(sigCh)
		close(stopSig)
	}()

	// Forward our stdin to the child; close the child's stdin on EOF.
	go func() {
		_, _ = io.Copy(stdinPipe, os.Stdin)
		_ = stdinPipe.Close()
	}()

	events := make(chan lineEvent, 64)
	go readStream(stdoutPipe, "stdout", events)
	go readStream(stderrPipe, "stderr", events)

	sendErr := driveIO(p, events, swallow, logFile)
	if sendErr == nil {
		sendErr = p.FlushPending()
	}

	waitErr := cmd.Wait()
	_ = waitErr // exit status read from ProcessState below

	return Result{ExitCode: normalizeExitCode(cmd.ProcessState), LastMessage: lastMessage}, sendErr
}

// driveIO consumes line events and the flush timer until both output streams
// report EOF. It returns the first send error encountered (strict mode).
func driveIO(p *stream.Processor, events <-chan lineEvent, swallow bool, logFile io.Writer) error {
	outputsOpen := 2
	for outputsOpen > 0 {
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
				outputsOpen--
				continue
			}
			if logFile != nil {
				_, _ = io.WriteString(logFile, ev.text)
			}
			if !swallow {
				out := os.Stdout
				if ev.streamName == "stderr" {
					out = os.Stderr
				}
				_, _ = io.WriteString(out, ev.text)
			}
			if err := p.ProcessLine(ev.text); err != nil {
				return err
			}
		case <-timerC:
			if err := p.FlushIfDue(); err != nil {
				return err
			}
		}
	}
	return nil
}

// readStream reads r line by line (preserving newlines), emitting one event per
// line and a final eof event. The trailing unterminated line, if any, is
// emitted before eof.
func readStream(r io.Reader, name string, events chan<- lineEvent) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			events <- lineEvent{streamName: name, text: line}
		}
		if err != nil {
			events <- lineEvent{streamName: name, eof: true}
			return
		}
	}
}
