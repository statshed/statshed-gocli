// Package output formats command results as plain text, JSON, or colored text.
//
// AIDEV-NOTE: PlainFormatter is always available and produces the same layout
// the Python CLI's PlainFormatter did (emoji status indicators). JSONFormatter
// emits indented JSON. ColorFormatter reuses the plain layout but adds ANSI
// color to status words/indicators; it is selected when output is a TTY (or
// color=always), replacing the Python Rich formatter.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Formatter renders API responses for human or machine consumption.
type Formatter interface {
	Health(data map[string]any) string
	Groups(data map[string]any) string
	Jobs(data map[string]any) string
	Config(data map[string]any) string
	GroupConfig(data map[string]any) string
	SubmitSuccess(data map[string]any) string
	Error(message string) string
}

// statusIndicators are emoji shown for each status in plain/color output.
var statusIndicators = map[string]string{
	"success":     "✅",
	"error":       "❌",
	"progress":    "🔄",
	"timeout":     "⏰",
	"stale":       "⚠️",
	"healthy":     "✅",
	"unhealthy":   "❌",
	"in_progress": "🔄",
	"empty":       "📭",
}

func indicator(status string) string {
	if v, ok := statusIndicators[status]; ok {
		return v
	}
	return "❓"
}

var statusOrder = []string{"success", "error", "progress", "timeout", "stale"}

// New returns the formatter for the given output format and color mode.
//
// outputFormat is "table" or "json"; color is "auto", "always", or "never".
func New(outputFormat, color string) Formatter {
	if outputFormat == "json" {
		return JSONFormatter{}
	}
	if ShouldUseColor(color) {
		return ColorFormatter{}
	}
	return PlainFormatter{}
}

// ShouldUseColor reports whether colored output should be used.
func ShouldUseColor(color string) bool {
	switch color {
	case "never":
		return false
	case "always":
		return true
	default:
		return isTerminal(os.Stdout)
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// --- value helpers -----------------------------------------------------------

func asMap(data map[string]any, key string) map[string]any {
	if v, ok := data[key].(map[string]any); ok {
		return v
	}
	return nil
}

func asSlice(data map[string]any, key string) []any {
	if v, ok := data[key].([]any); ok {
		return v
	}
	return nil
}

func str(data map[string]any, key, def string) string {
	if v, ok := data[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return def
}

// strFallback returns the first present key's string value, else def.
func strFallback(data map[string]any, def string, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	return def
}

// num renders a numeric value (JSON numbers decode as float64) without a
// trailing ".0", falling back through keys.
func num(data map[string]any, def string, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok && v != nil {
			return numString(v, def)
		}
	}
	return def
}

func numString(v any, def string) string {
	switch n := v.(type) {
	case float64:
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%g", n)
	case int:
		return fmt.Sprintf("%d", n)
	case int64:
		return fmt.Sprintf("%d", n)
	case nil:
		return def
	default:
		return fmt.Sprintf("%v", v)
	}
}

func countOf(counts map[string]any, key string) int {
	if v, ok := counts[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 0
}

// --- PlainFormatter ----------------------------------------------------------

// PlainFormatter renders dependency-free text with emoji status indicators.
type PlainFormatter struct{}

func (PlainFormatter) Health(data map[string]any) string {
	status := str(data, "status", "unknown")
	lines := []string{
		fmt.Sprintf("System Health: %s %s", indicator(status), strings.ToUpper(status)),
		"Total Jobs: " + num(data, "0", "total", "total_jobs"),
	}
	counts := firstMap(data, "counts", "by_status")
	if len(counts) > 0 {
		for _, s := range statusOrder {
			lines = append(lines, fmt.Sprintf("  %s: %d", title(s), countOf(counts, s)))
		}
	}
	return strings.Join(lines, "\n")
}

func (PlainFormatter) Groups(data map[string]any) string {
	groups := asSlice(data, "groups")
	if len(groups) == 0 {
		return "No groups found."
	}
	var lines []string
	for _, g := range groups {
		group, _ := g.(map[string]any)
		health := strFallback(group, "unknown", "health_status", "health")
		jobCount := num(group, "0", "job_count")
		lines = append(lines, fmt.Sprintf("%s %s (%s jobs)", indicator(health), str(group, "name", ""), jobCount))
		if summary := plainStatusSummary(asMap(group, "status_counts")); summary != "" {
			lines = append(lines, "    "+summary)
		}
	}
	return strings.Join(lines, "\n")
}

func (PlainFormatter) Jobs(data map[string]any) string {
	group := asMap(data, "group")
	lines := []string{"Group: " + str(group, "name", "unknown")}
	jobs := asSlice(data, "jobs")
	if len(jobs) == 0 {
		lines = append(lines, "  No jobs found.")
		return strings.Join(lines, "\n")
	}
	for _, j := range jobs {
		job, _ := j.(map[string]any)
		status := str(job, "status", "unknown")
		msg := str(job, "message", "")
		suffix := ""
		if msg != "" {
			suffix = " - " + msg
		}
		lines = append(lines, fmt.Sprintf("  %s %s: %s%s", indicator(status), str(job, "name", ""), status, suffix))
	}
	return strings.Join(lines, "\n")
}

func (PlainFormatter) Config(data map[string]any) string {
	return fmt.Sprintf(
		"Progress Timeout: %s minutes\nStaleness Timeout: %s hours",
		num(data, "N/A", "progress_timeout_minutes"),
		num(data, "N/A", "staleness_timeout_hours"))
}

func (PlainFormatter) GroupConfig(data map[string]any) string {
	lines := []string{"Group: " + str(data, "group", "unknown")}
	lines = append(lines, "  "+groupConfigLine(data, "Progress Timeout", "minutes",
		"progress_timeout_minutes", "effective_progress_timeout_minutes"))
	lines = append(lines, "  "+groupConfigLine(data, "Staleness Timeout", "hours",
		"staleness_timeout_hours", "effective_staleness_timeout_hours"))
	return strings.Join(lines, "\n")
}

func (PlainFormatter) SubmitSuccess(data map[string]any) string {
	job := asMap(data, "job")
	return fmt.Sprintf("Status submitted: %s/%s = %s",
		str(job, "group_name", "unknown"), str(job, "name", "unknown"), str(job, "status", "unknown"))
}

func (PlainFormatter) Error(message string) string {
	return "Error: " + message
}

// --- ColorFormatter ----------------------------------------------------------

// ANSI color codes used by ColorFormatter.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiCyan   = "\x1b[36m"
)

var statusColors = map[string]string{
	"success":     ansiGreen,
	"error":       ansiRed,
	"progress":    ansiBlue,
	"timeout":     ansiYellow,
	"stale":       ansiYellow,
	"healthy":     ansiGreen,
	"unhealthy":   ansiRed,
	"in_progress": ansiBlue,
	"empty":       ansiDim,
}

func colorize(text, color string) string {
	if color == "" {
		return text
	}
	return color + text + ansiReset
}

func statusColor(status string) string {
	if c, ok := statusColors[status]; ok {
		return c
	}
	return ""
}

// ColorFormatter renders the plain layout with ANSI color accents.
type ColorFormatter struct{}

func (ColorFormatter) Health(data map[string]any) string {
	status := str(data, "status", "unknown")
	header := colorize("System Health:", ansiBold) + " " +
		indicator(status) + " " + colorize(strings.ToUpper(status), statusColor(status))
	lines := []string{header, colorize("Total Jobs:", ansiDim) + " " + num(data, "0", "total", "total_jobs")}
	counts := firstMap(data, "counts", "by_status")
	if len(counts) > 0 {
		for _, s := range statusOrder {
			val := colorize(fmt.Sprintf("%d", countOf(counts, s)), statusColor(s))
			lines = append(lines, fmt.Sprintf("  %s: %s", title(s), val))
		}
	}
	return strings.Join(lines, "\n")
}

func (ColorFormatter) Groups(data map[string]any) string {
	groups := asSlice(data, "groups")
	if len(groups) == 0 {
		return colorize("No groups found.", ansiDim)
	}
	var lines []string
	for _, g := range groups {
		group, _ := g.(map[string]any)
		health := strFallback(group, "unknown", "health_status", "health")
		dot := colorize("●", statusColor(health))
		name := colorize(str(group, "name", ""), ansiBold)
		lines = append(lines, fmt.Sprintf("%s %s (%s jobs)", dot, name, num(group, "0", "job_count")))
		if summary := colorStatusSummary(asMap(group, "status_counts")); summary != "" {
			lines = append(lines, "    "+summary)
		}
	}
	return strings.Join(lines, "\n")
}

func (ColorFormatter) Jobs(data map[string]any) string {
	group := asMap(data, "group")
	lines := []string{colorize("Group:", ansiCyan) + " " + colorize(str(group, "name", "unknown"), ansiBold)}
	jobs := asSlice(data, "jobs")
	if len(jobs) == 0 {
		lines = append(lines, "  "+colorize("No jobs found.", ansiDim))
		return strings.Join(lines, "\n")
	}
	for _, j := range jobs {
		job, _ := j.(map[string]any)
		status := str(job, "status", "unknown")
		dot := colorize("●", statusColor(status))
		name := colorize(str(job, "name", ""), ansiBold)
		msg := str(job, "message", "")
		suffix := ""
		if msg != "" {
			suffix = " - " + msg
		}
		lines = append(lines, fmt.Sprintf("  %s %s: %s%s", dot, name, colorize(status, statusColor(status)), suffix))
	}
	return strings.Join(lines, "\n")
}

func (ColorFormatter) Config(data map[string]any) string {
	return colorize("Progress Timeout:", ansiCyan) + " " + num(data, "N/A", "progress_timeout_minutes") + " minutes\n" +
		colorize("Staleness Timeout:", ansiCyan) + " " + num(data, "N/A", "staleness_timeout_hours") + " hours"
}

func (ColorFormatter) GroupConfig(data map[string]any) string {
	lines := []string{colorize("Group:", ansiCyan) + " " + colorize(str(data, "group", "unknown"), ansiBold)}
	lines = append(lines, "  "+groupConfigLine(data, "Progress Timeout", "minutes",
		"progress_timeout_minutes", "effective_progress_timeout_minutes"))
	lines = append(lines, "  "+groupConfigLine(data, "Staleness Timeout", "hours",
		"staleness_timeout_hours", "effective_staleness_timeout_hours"))
	return strings.Join(lines, "\n")
}

func (ColorFormatter) SubmitSuccess(data map[string]any) string {
	job := asMap(data, "job")
	status := str(job, "status", "unknown")
	return colorize("✓ ", ansiGreen+ansiBold) + colorize("Status submitted: ", ansiDim) +
		colorize(fmt.Sprintf("%s/%s", str(job, "group_name", "unknown"), str(job, "name", "unknown")), ansiBold) +
		colorize(" → ", ansiDim) + colorize(status, statusColor(status))
}

func (ColorFormatter) Error(message string) string {
	return colorize("Error: ", ansiRed+ansiBold) + colorize(message, ansiRed)
}

// --- JSONFormatter -----------------------------------------------------------

// JSONFormatter emits indented JSON for machine consumption.
type JSONFormatter struct{}

func (JSONFormatter) format(data any) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": %q}", err.Error())
	}
	return string(b)
}

func (f JSONFormatter) Health(data map[string]any) string        { return f.format(data) }
func (f JSONFormatter) Groups(data map[string]any) string        { return f.format(data) }
func (f JSONFormatter) Jobs(data map[string]any) string          { return f.format(data) }
func (f JSONFormatter) Config(data map[string]any) string        { return f.format(data) }
func (f JSONFormatter) GroupConfig(data map[string]any) string   { return f.format(data) }
func (f JSONFormatter) SubmitSuccess(data map[string]any) string { return f.format(data) }
func (f JSONFormatter) Error(message string) string {
	return f.format(map[string]any{"error": message})
}

// --- shared helpers ----------------------------------------------------------

func firstMap(data map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		if m := asMap(data, k); m != nil {
			return m
		}
	}
	return nil
}

func plainStatusSummary(counts map[string]any) string {
	if counts == nil {
		return ""
	}
	var parts []string
	for _, s := range statusOrder {
		if c := countOf(counts, s); c > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", s, c))
		}
	}
	return strings.Join(parts, ", ")
}

func colorStatusSummary(counts map[string]any) string {
	if counts == nil {
		return ""
	}
	var parts []string
	for _, s := range statusOrder {
		if c := countOf(counts, s); c > 0 {
			parts = append(parts, colorize(fmt.Sprintf("%s: %d", s, c), statusColor(s)))
		}
	}
	return strings.Join(parts, ", ")
}

// groupConfigLine renders one effective/override config row. When the override
// key is absent the effective value is shown with "(global default)".
func groupConfigLine(data map[string]any, label, unit, overrideKey, effectiveKey string) string {
	if v, ok := data[overrideKey]; ok && v != nil {
		return fmt.Sprintf("%s: %s %s (override)", label, numString(v, "N/A"), unit)
	}
	return fmt.Sprintf("%s: %s %s (global default)", label, num(data, "N/A", effectiveKey), unit)
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
