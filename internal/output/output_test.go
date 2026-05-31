package output

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlainHealth(t *testing.T) {
	data := map[string]any{
		"status": "healthy",
		"total":  float64(3),
		"counts": map[string]any{"success": float64(2), "progress": float64(1)},
	}
	got := PlainFormatter{}.Health(data)
	want := "System Health: ✅ HEALTHY\nTotal Jobs: 3\n  Success: 2\n  Error: 0\n  Progress: 1\n  Timeout: 0\n  Stale: 0"
	if got != want {
		t.Fatalf("Health =\n%q\nwant\n%q", got, want)
	}
}

func TestPlainGroupsEmpty(t *testing.T) {
	if got := (PlainFormatter{}).Groups(map[string]any{"groups": []any{}}); got != "No groups found." {
		t.Fatalf("empty groups = %q", got)
	}
}

func TestPlainJobs(t *testing.T) {
	data := map[string]any{
		"group": map[string]any{"name": "ci"},
		"jobs":  []any{map[string]any{"name": "build", "status": "success", "message": "ok"}},
	}
	got := PlainFormatter{}.Jobs(data)
	want := "Group: ci\n  ✅ build: success - ok"
	if got != want {
		t.Fatalf("Jobs =\n%q\nwant\n%q", got, want)
	}
}

func TestPlainSubmitSuccess(t *testing.T) {
	data := map[string]any{"job": map[string]any{"group_name": "g", "name": "j", "status": "success"}}
	if got := (PlainFormatter{}).SubmitSuccess(data); got != "Status submitted: g/j = success" {
		t.Fatalf("SubmitSuccess = %q", got)
	}
}

func TestGroupConfigOverrideVsDefault(t *testing.T) {
	// Override present.
	data := map[string]any{
		"group":                             "ci",
		"progress_timeout_minutes":          float64(15),
		"effective_staleness_timeout_hours": float64(24),
	}
	got := PlainFormatter{}.GroupConfig(data)
	if !strings.Contains(got, "Progress Timeout: 15 minutes (override)") {
		t.Fatalf("expected override line, got:\n%s", got)
	}
	if !strings.Contains(got, "Staleness Timeout: 24 hours (global default)") {
		t.Fatalf("expected default line, got:\n%s", got)
	}
}

func TestJSONFormatterRoundTrips(t *testing.T) {
	data := map[string]any{"status": "healthy", "n": float64(1)}
	out := JSONFormatter{}.Health(data)
	var back map[string]any
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if back["status"] != "healthy" {
		t.Fatalf("round trip lost data: %v", back)
	}
}

func TestColorFormatterContainsANSI(t *testing.T) {
	data := map[string]any{"status": "unhealthy", "total": float64(0)}
	got := ColorFormatter{}.Health(data)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("color output should contain ANSI codes: %q", got)
	}
}

func TestShouldUseColor(t *testing.T) {
	if ShouldUseColor("never") {
		t.Fatal("never should be false")
	}
	if !ShouldUseColor("always") {
		t.Fatal("always should be true")
	}
}

func TestNewSelectsFormatter(t *testing.T) {
	if _, ok := New("json", "auto").(JSONFormatter); !ok {
		t.Fatal("json format should give JSONFormatter")
	}
	if _, ok := New("table", "never").(PlainFormatter); !ok {
		t.Fatal("table+never should give PlainFormatter")
	}
	if _, ok := New("table", "always").(ColorFormatter); !ok {
		t.Fatal("table+always should give ColorFormatter")
	}
}
