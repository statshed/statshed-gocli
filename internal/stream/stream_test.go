package stream

import (
	"reflect"
	"testing"
	"time"
)

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mred\x1b[0m text"
	if got := StripANSI(in); got != "red text" {
		t.Fatalf("StripANSI = %q, want %q", got, "red text")
	}
}

func TestCleanLine(t *testing.T) {
	if got := CleanLine("  \x1b[1mhi\x1b[0m  \n"); got != "hi" {
		t.Fatalf("CleanLine = %q, want %q", got, "hi")
	}
}

// fakeClock returns a controllable time source.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time    { return c.t }
func (c *fakeClock) advance(d float64) { c.t = c.t.Add(time.Duration(d * float64(time.Second))) }

func collector() (*[]string, SendFunc) {
	var got []string
	return &got, func(m string) error { got = append(got, m); return nil }
}

func TestDebounceFirstImmediateThenPending(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	got, send := collector()
	p := New(60, nil, nil, send).WithClock(clk.now)

	if err := p.ProcessLine("first\n"); err != nil {
		t.Fatal(err)
	}
	// Within the window: held as pending, not sent.
	clk.advance(10)
	if err := p.ProcessLine("second\n"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, []string{"first"}) {
		t.Fatalf("after window-internal line got %v, want [first]", *got)
	}
	// Last-wins: another line overwrites pending.
	if err := p.ProcessLine("third\n"); err != nil {
		t.Fatal(err)
	}
	// Advance past the window and flush.
	clk.advance(60)
	if err := p.FlushIfDue(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, []string{"first", "third"}) {
		t.Fatalf("got %v, want [first third]", *got)
	}
}

func TestFlushPendingOnEOF(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	got, send := collector()
	p := New(60, nil, nil, send).WithClock(clk.now)
	_ = p.ProcessLine("a")
	_ = p.ProcessLine("b") // pending
	if err := p.FlushPending(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, []string{"a", "b"}) {
		t.Fatalf("got %v, want [a b]", *got)
	}
}

func TestBlankAndWhitespaceLinesSkipped(t *testing.T) {
	got, send := collector()
	p := New(0, nil, nil, send)
	_ = p.ProcessLine("   \n")
	_ = p.ProcessLine("\x1b[0m\n")
	if len(*got) != 0 {
		t.Fatalf("blank lines should be skipped, got %v", *got)
	}
}

func TestIncludeExcludeFilters(t *testing.T) {
	inc, err := CompilePatterns([]string{"keep"}, false)
	if err != nil {
		t.Fatal(err)
	}
	exc, err := CompilePatterns([]string{"drop"}, false)
	if err != nil {
		t.Fatal(err)
	}
	got, send := collector()
	p := New(0, inc, exc, send)
	_ = p.ProcessLine("please keep this")
	_ = p.ProcessLine("ignore me") // not matched by include
	_ = p.ProcessLine("keep but drop this")
	if !reflect.DeepEqual(*got, []string{"please keep this"}) {
		t.Fatalf("got %v, want [please keep this]", *got)
	}
}

func TestIgnoreCase(t *testing.T) {
	inc, err := CompilePatterns([]string{"ERROR"}, true)
	if err != nil {
		t.Fatal(err)
	}
	got, send := collector()
	p := New(0, inc, nil, send)
	_ = p.ProcessLine("an error occurred")
	if !reflect.DeepEqual(*got, []string{"an error occurred"}) {
		t.Fatalf("got %v, want match via case-insensitive", *got)
	}
}

func TestCompilePatternsError(t *testing.T) {
	if _, err := CompilePatterns([]string{"("}, false); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
