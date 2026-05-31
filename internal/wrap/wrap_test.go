package wrap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/statshed/statshed-cli/internal/stream"
)

func TestRunCapturesOutputAndExitCode(t *testing.T) {
	var sent []string
	p := stream.New(0, nil, nil, func(m string) error {
		sent = append(sent, m)
		return nil
	})

	var logBuf bytes.Buffer
	res, err := Run([]string{"sh", "-c", "echo hello; echo world; exit 0"}, p, true, &logBuf)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", res.ExitCode)
	}
	if res.LastMessage != "world" {
		t.Fatalf("last message = %q, want world", res.LastMessage)
	}
	joined := strings.Join(sent, ",")
	if !strings.Contains(joined, "hello") || !strings.Contains(joined, "world") {
		t.Fatalf("expected hello and world in %v", sent)
	}
	if !strings.Contains(logBuf.String(), "hello") {
		t.Fatalf("log file should capture output: %q", logBuf.String())
	}
}

func TestRunPropagatesNonZeroExit(t *testing.T) {
	p := stream.New(0, nil, nil, func(string) error { return nil })
	res, err := Run([]string{"sh", "-c", "exit 7"}, p, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", res.ExitCode)
	}
}

func TestRunMissingBinary(t *testing.T) {
	p := stream.New(0, nil, nil, func(string) error { return nil })
	_, err := Run([]string{"/no/such/binary-xyz"}, p, true, nil)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestSignalDeathExitCode(t *testing.T) {
	p := stream.New(0, nil, nil, func(string) error { return nil })
	// kill -TERM self → 128 + 15 = 143.
	res, err := Run([]string{"sh", "-c", "kill -TERM $$"}, p, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 143 {
		t.Fatalf("exit code = %d, want 143", res.ExitCode)
	}
}
