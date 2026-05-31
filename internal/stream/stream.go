// Package stream implements line filtering and debounced submission shared by
// the `stream` and `wrap` commands.
//
// AIDEV-NOTE: This is the pure state machine, kept IO-free so it can be unit
// tested without real IO or a real clock. Inject a clock via WithClock.
//
// Behavior:
//   - Every accepted line is submitted as a "progress" status message.
//   - Leading/trailing whitespace and ANSI escape sequences are stripped.
//   - regex patterns are an include filter (at least one must match the cleaned
//     line). ignore patterns are an exclude filter applied after.
//   - The first accepted line is sent immediately. Subsequent lines within the
//     minTime window are stored as a "pending" message using last-wins
//     semantics; when the window expires the pending message is flushed.
//   - On EOF any pending message is flushed unconditionally.
package stream

import (
	"regexp"
	"strings"
	"time"
)

// ansiEscape matches the ECMA-48 escape sequences we scrub from log lines
// (CSI "ESC [ ... final-byte" plus the short single-character escapes).
var ansiEscape = regexp.MustCompile("\x1b(?:[@-Z\\\\-_]|\\[[0-?]*[ -/]*[@-~])")

// StripANSI removes ANSI escape sequences from text.
func StripANSI(text string) string {
	return ansiEscape.ReplaceAllString(text, "")
}

// CleanLine strips ANSI codes and surrounding whitespace from a raw line.
func CleanLine(raw string) string {
	return strings.TrimSpace(StripANSI(raw))
}

// SendFunc submits a single status message.
type SendFunc func(message string) error

// Processor is a debounced, filtered status-message submitter.
//
// AIDEV-NOTE: Intentionally IO-free. Send is called to submit a message; the
// caller handles errors and drives TimeUntilNextFlush/FlushIfDue on real time.
type Processor struct {
	minTime  time.Duration
	includes []*regexp.Regexp
	excludes []*regexp.Regexp
	send     SendFunc
	now      func() time.Time

	lastSend    time.Time
	hasLastSend bool
	pending     string
	hasPending  bool
}

// New creates a Processor. minTime is the debounce window in seconds.
func New(minTimeSeconds float64, includes, excludes []*regexp.Regexp, send SendFunc) *Processor {
	return &Processor{
		minTime:  time.Duration(minTimeSeconds * float64(time.Second)),
		includes: includes,
		excludes: excludes,
		send:     send,
		now:      time.Now,
	}
}

// WithClock overrides the time source (for tests). Returns the receiver.
func (p *Processor) WithClock(now func() time.Time) *Processor {
	p.now = now
	return p
}

// SetSend replaces the send function (used by wrap to wrap it for capture).
func (p *Processor) SetSend(send SendFunc) { p.send = send }

// Send returns the current send function.
func (p *Processor) Send() SendFunc { return p.send }

// ProcessLine processes a single raw input line; it may call Send.
func (p *Processor) ProcessLine(rawLine string) error {
	cleaned := CleanLine(rawLine)
	if cleaned == "" {
		return nil
	}
	if len(p.includes) > 0 && !anyMatch(p.includes, cleaned) {
		return nil
	}
	if anyMatch(p.excludes, cleaned) {
		return nil
	}

	now := p.now()
	if !p.hasLastSend || now.Sub(p.lastSend) >= p.minTime {
		return p.doSend(cleaned, now)
	}
	// Last-wins debounce: newer messages overwrite older ones in the window.
	p.pending = cleaned
	p.hasPending = true
	return nil
}

// TimeUntilNextFlush returns the duration until a pending message is due, and
// ok=false when there is nothing pending.
func (p *Processor) TimeUntilNextFlush() (d time.Duration, ok bool) {
	if !p.hasPending || !p.hasLastSend {
		return 0, false
	}
	elapsed := p.now().Sub(p.lastSend)
	remaining := p.minTime - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}

// FlushIfDue flushes the pending message if the minTime window has elapsed.
func (p *Processor) FlushIfDue() error {
	if !p.hasPending {
		return nil
	}
	now := p.now()
	if !p.hasLastSend || now.Sub(p.lastSend) >= p.minTime {
		return p.doSend(p.pending, now)
	}
	return nil
}

// FlushPending unconditionally flushes any pending message (used on EOF).
func (p *Processor) FlushPending() error {
	if p.hasPending {
		return p.doSend(p.pending, p.now())
	}
	return nil
}

func (p *Processor) doSend(message string, now time.Time) error {
	if err := p.send(message); err != nil {
		return err
	}
	p.lastSend = now
	p.hasLastSend = true
	p.pending = ""
	p.hasPending = false
	return nil
}

func anyMatch(patterns []*regexp.Regexp, s string) bool {
	for _, re := range patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// CompilePatterns compiles user-supplied regex strings. When ignoreCase is set
// each pattern is compiled case-insensitively. Returns the first error.
func CompilePatterns(patterns []string, ignoreCase bool) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		expr := p
		if ignoreCase {
			expr = "(?i)" + p
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}
