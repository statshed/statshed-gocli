package errors

import (
	"fmt"
	"testing"
)

func TestExitCodes(t *testing.T) {
	cases := []struct {
		err  *StatShedError
		code ExitCode
	}{
		{Config("x"), ExitConfig},
		{Connection("x"), ExitConnection},
		{Timeout("x"), ExitTimeout},
		{API("x"), ExitAPI},
		{NotFound("x"), ExitNotFound},
		{InvalidArgs("x"), ExitInvalidArgs},
	}
	for _, c := range cases {
		if c.err.Code != c.code {
			t.Errorf("%v: code = %d, want %d", c.err, c.err.Code, c.code)
		}
		if ExitCodeOf(c.err) != c.code {
			t.Errorf("ExitCodeOf(%v) = %d, want %d", c.err, ExitCodeOf(c.err), c.code)
		}
	}
}

func TestExitCodeOfPlainError(t *testing.T) {
	if got := ExitCodeOf(fmt.Errorf("boom")); got != ExitAPI {
		t.Fatalf("plain error should map to ExitAPI, got %d", got)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(NotFound("nope")) {
		t.Fatal("IsNotFound should be true for NotFound")
	}
	if IsNotFound(API("x")) {
		t.Fatal("IsNotFound should be false for API error")
	}
}

func TestWrappedError(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", NotFound("inner"))
	if !IsNotFound(wrapped) {
		t.Fatal("IsNotFound should unwrap")
	}
	if ExitCodeOf(wrapped) != ExitNotFound {
		t.Fatalf("ExitCodeOf should unwrap, got %d", ExitCodeOf(wrapped))
	}
}
