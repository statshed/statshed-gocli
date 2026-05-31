package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// runCLI executes the CLI with the given args against the given base URL,
// capturing stdout and returning it along with the exit code. Each call builds
// a fresh root command so global flag state does not leak between cases.
func runCLI(t *testing.T, baseURL string, args ...string) (string, int) {
	t.Helper()
	t.Setenv("STATSHED_URL", baseURL)
	t.Setenv("STATSHED_CONFIG", "")

	oldArgs := os.Args
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Args = append([]string{"statshed"}, args...)
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	}()

	code := Execute("test")
	_ = w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = oldStdout
	return string(out), code
}

func TestHealthHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"healthy","total":1,"counts":{"success":1}}`)
	}))
	defer srv.Close()

	out, code := runCLI(t, srv.URL, "--no-color", "health")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if want := "System Health: ✅ HEALTHY"; !strings.Contains(out, want) {
		t.Fatalf("output %q missing %q", out, want)
	}
}

func TestHealthUnhealthyExit1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"unhealthy","total":1,"counts":{"error":1}}`)
	}))
	defer srv.Close()

	_, code := runCLI(t, srv.URL, "--no-color", "health")
	if code != 1 {
		t.Fatalf("unhealthy exit code = %d, want 1", code)
	}
}

func TestSubmitLenientSwallowsError(t *testing.T) {
	// No server: connection refused. Lenient submit must still exit 0.
	_, code := runCLI(t, "http://127.0.0.1:1", "--quiet", "submit", "-g", "g", "-j", "j", "-s", "success")
	if code != 0 {
		t.Fatalf("lenient submit exit = %d, want 0", code)
	}
}

func TestSubmitStrictPropagatesError(t *testing.T) {
	_, code := runCLI(t, "http://127.0.0.1:1", "--quiet", "submit", "--strict", "-g", "g", "-j", "j", "-s", "success")
	if code != 3 {
		t.Fatalf("strict submit exit = %d, want 3 (connection)", code)
	}
}

func TestInvalidStatusExit10(t *testing.T) {
	_, code := runCLI(t, "http://127.0.0.1:1", "submit", "-g", "g", "-j", "j", "-s", "bogus")
	if code != 10 {
		t.Fatalf("invalid status exit = %d, want 10", code)
	}
}

func TestJobsNotFoundExit11(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"error":"missing"}`)
	}))
	defer srv.Close()

	_, code := runCLI(t, srv.URL, "--no-color", "jobs", "nope")
	if code != 11 {
		t.Fatalf("not found exit = %d, want 11", code)
	}
}

func TestBadConfigExit5(t *testing.T) {
	_, code := runCLI(t, "http://127.0.0.1:1", "--config", "/no/such/file.yaml", "health")
	if code != 5 {
		t.Fatalf("bad config exit = %d, want 5", code)
	}
}
