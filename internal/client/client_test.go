package client

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	sserr "github.com/statshed/statshed-cli/internal/errors"
)

func newTestClient(url string) *Client {
	return New(url, 5, 0, 0)
}

func TestGetHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"healthy"}`)
	}))
	defer srv.Close()

	data, err := newTestClient(srv.URL).GetHealth()
	if err != nil {
		t.Fatal(err)
	}
	if data["status"] != "healthy" {
		t.Fatalf("got %v", data)
	}
}

func TestSubmitStatusJSON(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s", ct)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = io.WriteString(w, `{"job":{"status":"success"}}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).SubmitStatus("g", "j", "success", "hi", "")
	if err != nil {
		t.Fatal(err)
	}
	if body["group"] != "g" || body["job"] != "j" || body["status"] != "success" || body["message"] != "hi" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestSubmitStatusMultipart(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "build.log")
	if err := os.WriteFile(logPath, []byte("log contents"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotFile, gotStatus string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mt != "multipart/form-data" {
			t.Errorf("expected multipart, got %s", mt)
		}
		_ = r.ParseMultipartForm(1 << 20)
		gotStatus = r.FormValue("status")
		f, hdr, err := r.FormFile("log")
		if err != nil {
			t.Errorf("missing log file: %v", err)
		} else {
			defer f.Close()
			gotFile = hdr.Filename
		}
		_, _ = io.WriteString(w, `{"job":{"status":"error"}}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).SubmitStatus("g", "j", "error", "", logPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotStatus != "error" || gotFile != "build.log" {
		t.Fatalf("status=%q file=%q", gotStatus, gotFile)
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"error":"group missing"}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetJobs("nope")
	if !sserr.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	if !strings.Contains(err.Error(), "group missing") {
		t.Fatalf("error message not propagated: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, `{"error":"boom"}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetHealth()
	if sserr.ExitCodeOf(err) != sserr.ExitAPI {
		t.Fatalf("expected API exit code, got %d (%v)", sserr.ExitCodeOf(err), err)
	}
}

func TestConnectionError(t *testing.T) {
	// Nothing listening on this port.
	_, err := New("http://127.0.0.1:1", 1, 0, 0).GetHealth()
	if sserr.ExitCodeOf(err) != sserr.ExitConnection {
		t.Fatalf("expected connection exit code, got %d (%v)", sserr.ExitCodeOf(err), err)
	}
}

func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			// Simulate a transient failure by hijacking and closing the conn.
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
				return
			}
		}
		_, _ = io.WriteString(w, `{"status":"healthy"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, 5, 3, 0) // retryDelay 0 to keep the test fast
	data, err := c.GetHealth()
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if data["status"] != "healthy" {
		t.Fatalf("unexpected data: %v", data)
	}
	if attempts.Load() < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", attempts.Load())
	}
}

func TestUpdateGroupConfigReset(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = io.WriteString(w, `{"group":"ci"}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).UpdateGroupConfig("ci", nil, nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	// Reset sends an explicit null for progress.
	if v, ok := body["progress_timeout_minutes"]; !ok || v != nil {
		t.Fatalf("expected explicit null progress timeout, body=%v", body)
	}
	if _, ok := body["staleness_timeout_hours"]; ok {
		t.Fatalf("staleness should be omitted when not changed/reset, body=%v", body)
	}
}
