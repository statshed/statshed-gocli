// Package client is the HTTP client for the StatShed backend.
//
// AIDEV-NOTE: All API interactions go through Client. HTTP/transport errors are
// converted to *errors.StatShedError with appropriate exit codes. Retry logic
// handles transient failures (connection errors, timeouts) with exponential
// backoff and jitter when Retries > 0. POST/PUT retries are safe because the
// /status endpoint uses upsert (idempotent) semantics.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	sserr "github.com/statshed/statshed-cli/internal/errors"
)

// Client interacts with the StatShed API.
type Client struct {
	BaseURL    string
	Timeout    time.Duration
	Retries    int
	RetryDelay time.Duration
	HTTPClient *http.Client
}

// New constructs a Client. timeout is in seconds, retryDelay in seconds.
func New(baseURL string, timeout, retries int, retryDelay float64) *Client {
	to := time.Duration(timeout) * time.Second
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Timeout:    to,
		Retries:    retries,
		RetryDelay: time.Duration(retryDelay * float64(time.Second)),
		HTTPClient: &http.Client{Timeout: to},
	}
}

func (c *Client) url(path string) string {
	return c.BaseURL + "/" + strings.TrimLeft(path, "/")
}

// JSON is a decoded JSON object response.
type JSON = map[string]any

// doJSON performs a JSON request with retry logic and returns the parsed body.
func (c *Client) doJSON(method, path string, body any) (JSON, error) {
	target := c.url(path)

	build := func() (*http.Request, error) {
		var rdr io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, sserr.API("Failed to encode request: %v", err)
			}
			rdr = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, target, rdr)
		if err != nil {
			return nil, sserr.API("Failed to build request: %v", err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}

	resp, err := c.send(target, build)
	if err != nil {
		return nil, err
	}
	return c.parse(resp)
}

// send runs the retry loop. build is called fresh for each attempt so request
// bodies (and file handles, for multipart) are valid on every retry.
func (c *Client) send(target string, build func() (*http.Request, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		resp, err := c.HTTPClient.Do(req)
		if err == nil {
			return resp, nil
		}

		// Classify the transport error. Non-transient errors are not retried.
		if isTimeout(err) {
			lastErr = sserr.Timeout("Request to %s timed out after %s", target, c.Timeout)
		} else if isConnError(err) {
			lastErr = sserr.Connection("Could not connect to %s: %v", c.BaseURL, err)
		} else {
			return nil, sserr.API("Request failed: %v", err)
		}

		if attempt < c.Retries {
			base := c.RetryDelay * time.Duration(1<<attempt)
			jitter := time.Duration(rand.Float64() * float64(c.RetryDelay))
			time.Sleep(base + jitter)
		}
	}
	return nil, lastErr
}

// parse handles HTTP status codes and decodes the JSON body.
func (c *Client) parse(resp *http.Response) (JSON, error) {
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, sserr.NotFound("%s", errorMessage(data, "Resource not found"))
	}
	if resp.StatusCode >= 400 {
		fallback := fmt.Sprintf("API error (HTTP %d)", resp.StatusCode)
		return nil, sserr.APIStatus(resp.StatusCode, "%s", errorMessage(data, fallback))
	}

	var out JSON
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, sserr.API("Invalid JSON response from server: %v", err)
	}
	return out, nil
}

// errorMessage extracts the "error" field from a JSON error body, or returns
// fallback if absent/unparseable.
func errorMessage(data []byte, fallback string) string {
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err == nil && body.Error != "" {
		return body.Error
	}
	return fallback
}

func isTimeout(err error) bool {
	var te interface{ Timeout() bool }
	if errors.As(err, &te) && te.Timeout() {
		return true
	}
	return errors.Is(err, os.ErrDeadlineExceeded)
}

func isConnError(err error) bool {
	// AIDEV-NOTE: Anything that's a transport-level *url.Error and not a
	// timeout is treated as a connection failure (DNS, refused, reset).
	var ue *url.Error
	return errors.As(err, &ue)
}

// GetHealth returns the overall system health.
func (c *Client) GetHealth() (JSON, error) {
	return c.doJSON(http.MethodGet, "/health", nil)
}

// SubmitStatus submits a job status update. When logPath is non-empty the
// request is sent as multipart/form-data to carry the log file.
func (c *Client) SubmitStatus(group, job, status, message, logPath string) (JSON, error) {
	if logPath != "" {
		return c.submitWithLog(group, job, status, message, logPath)
	}
	payload := JSON{"group": group, "job": job, "status": status}
	if message != "" {
		payload["message"] = message
	}
	return c.doJSON(http.MethodPost, "/status", payload)
}

// submitWithLog uploads the status plus a log file via multipart/form-data.
func (c *Client) submitWithLog(group, job, status, message, logPath string) (JSON, error) {
	target := c.url("/status")

	build := func() (*http.Request, error) {
		f, err := os.Open(logPath)
		if err != nil {
			return nil, sserr.API("Cannot open log file %s: %v", logPath, err)
		}
		defer f.Close()

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("group", group)
		_ = mw.WriteField("job", job)
		_ = mw.WriteField("status", status)
		if message != "" {
			_ = mw.WriteField("message", message)
		}
		// Use the original filename for backend display/auditing.
		part, err := mw.CreateFormFile("log", filepath.Base(logPath))
		if err != nil {
			return nil, sserr.API("Failed to build upload: %v", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			return nil, sserr.API("Failed to read log file: %v", err)
		}
		if err := mw.Close(); err != nil {
			return nil, sserr.API("Failed to build upload: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, target, &buf)
		if err != nil {
			return nil, sserr.API("Failed to build request: %v", err)
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		return req, nil
	}

	resp, err := c.send(target, build)
	if err != nil {
		return nil, err
	}
	return c.parse(resp)
}

// GetGroups returns all groups with health summaries.
func (c *Client) GetGroups() (JSON, error) {
	return c.doJSON(http.MethodGet, "/groups", nil)
}

// GetJobs returns all jobs in a group.
func (c *Client) GetJobs(groupName string) (JSON, error) {
	return c.doJSON(http.MethodGet, "/groups/"+url.PathEscape(groupName)+"/jobs", nil)
}

// GetConfig returns the global configuration.
func (c *Client) GetConfig() (JSON, error) {
	return c.doJSON(http.MethodGet, "/config", nil)
}

// UpdateConfig updates the global configuration. nil pointers are omitted.
func (c *Client) UpdateConfig(progressTimeoutMinutes, stalenessTimeoutHours *int) (JSON, error) {
	payload := JSON{}
	if progressTimeoutMinutes != nil {
		payload["progress_timeout_minutes"] = *progressTimeoutMinutes
	}
	if stalenessTimeoutHours != nil {
		payload["staleness_timeout_hours"] = *stalenessTimeoutHours
	}
	return c.doJSON(http.MethodPut, "/config", payload)
}

// GetGroupConfig returns a group's configuration with effective values.
func (c *Client) GetGroupConfig(groupName string) (JSON, error) {
	return c.doJSON(http.MethodGet, "/groups/"+url.PathEscape(groupName)+"/config", nil)
}

// UpdateGroupConfig updates a group's config. reset* flags send null to revert
// to the global default; otherwise non-nil values are applied.
func (c *Client) UpdateGroupConfig(
	groupName string,
	progressTimeoutMinutes, stalenessTimeoutHours *int,
	resetProgress, resetStaleness bool,
) (JSON, error) {
	payload := JSON{}
	if resetProgress {
		payload["progress_timeout_minutes"] = nil
	} else if progressTimeoutMinutes != nil {
		payload["progress_timeout_minutes"] = *progressTimeoutMinutes
	}
	if resetStaleness {
		payload["staleness_timeout_hours"] = nil
	} else if stalenessTimeoutHours != nil {
		payload["staleness_timeout_hours"] = *stalenessTimeoutHours
	}
	return c.doJSON(http.MethodPut, "/groups/"+url.PathEscape(groupName)+"/config", payload)
}
