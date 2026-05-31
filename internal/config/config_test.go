package config

import (
	"os"
	"path/filepath"
	"testing"

	sserr "github.com/statshed/statshed-cli/internal/errors"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "statshed.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaults(t *testing.T) {
	t.Setenv("STATSHED_URL", "")
	t.Setenv("STATSHED_CONFIG", "")
	// Run in an empty dir so ./statshed.yaml isn't picked up.
	chdir(t, t.TempDir())
	cfg, err := FromSources("", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != DefaultURL || cfg.OutputFormat != "table" || cfg.Color != "auto" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestFilePrecedenceAndEnvAndCLI(t *testing.T) {
	p := writeTemp(t, "url: http://file:1\noutput_format: json\ntimeout: 30\nretries: 2\n")
	t.Setenv("STATSHED_CONFIG", p)

	// Env URL overrides file.
	t.Setenv("STATSHED_URL", "http://env:2")
	cfg, err := FromSources("", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "http://env:2" {
		t.Fatalf("env should override file: %s", cfg.URL)
	}
	if cfg.Timeout != 30 || cfg.Retries != 2 {
		t.Fatalf("file values not loaded: %+v", cfg)
	}

	// CLI URL overrides env.
	cfg, err = FromSources("", "http://cli:3", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "http://cli:3" {
		t.Fatalf("cli should override env: %s", cfg.URL)
	}
}

func TestCLIFlagsForColorAndJSON(t *testing.T) {
	chdir(t, t.TempDir())
	t.Setenv("STATSHED_CONFIG", "")
	cfg, err := FromSources("", "", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Color != "never" {
		t.Fatalf("no-color should set color=never, got %s", cfg.Color)
	}
	if cfg.OutputFormat != "json" {
		t.Fatalf("json flag should set output_format=json, got %s", cfg.OutputFormat)
	}
}

func TestColorBool(t *testing.T) {
	p := writeTemp(t, "color: true\n")
	cfg, err := loadConfigFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Color != "always" {
		t.Fatalf("color: true should map to always, got %s", cfg.Color)
	}

	p = writeTemp(t, "color: false\n")
	cfg, err = loadConfigFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Color != "never" {
		t.Fatalf("color: false should map to never, got %s", cfg.Color)
	}
}

func TestSubmitSection(t *testing.T) {
	p := writeTemp(t, "submit:\n  syslog: true\n  syslog_facility: local0\n  strict: true\n")
	cfg, err := loadConfigFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Submit.Syslog || cfg.Submit.SyslogFacility != "local0" || !cfg.Submit.Strict {
		t.Fatalf("submit section not parsed: %+v", cfg.Submit)
	}
}

func TestInvalidValues(t *testing.T) {
	cases := []string{
		"output_format: xml\n",
		"timeout: 0\n",
		"timeout: -5\n",
		"retries: -1\n",
		"color: maybe\n",
	}
	for _, c := range cases {
		p := writeTemp(t, c)
		if _, err := loadConfigFile(p); err == nil {
			t.Fatalf("expected error for config %q", c)
		}
	}
}

func TestMissingExplicitFileIsConfigError(t *testing.T) {
	_, err := FromSources("/no/such/file.yaml", "", false, false)
	if err == nil {
		t.Fatal("expected error for missing explicit config")
	}
	if sserr.ExitCodeOf(err) != sserr.ExitConfig {
		t.Fatalf("expected config exit code, got %d", sserr.ExitCodeOf(err))
	}
}

func TestTopLevelScalarRejected(t *testing.T) {
	p := writeTemp(t, "just a string\n")
	if _, err := loadConfigFile(p); err == nil {
		t.Fatal("expected error for non-mapping config")
	}
}

// chdir changes to dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
