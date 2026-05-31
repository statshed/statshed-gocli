// Package config handles loading and merging StatShed CLI configuration.
//
// AIDEV-NOTE: Configuration precedence (lowest to highest):
//  1. Built-in defaults
//  2. Config file (discovered or specified)
//  3. Environment variables
//  4. CLI arguments
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/statshed/statshed-cli/internal/errors"
)

// Default configuration values.
const (
	DefaultURL          = "http://localhost:7828"
	DefaultOutputFormat = "table"
	DefaultColor        = "auto"
	DefaultTimeout      = 10
	DefaultRetries      = 0
	DefaultRetryDelay   = 1.0
)

// SubmitConfig holds submit-command configuration.
type SubmitConfig struct {
	Syslog         bool
	SyslogFacility string
	Strict         bool
}

// Config holds all configuration values merged from defaults, file, env, CLI.
type Config struct {
	URL          string
	OutputFormat string
	Color        string
	Timeout      int
	Retries      int
	RetryDelay   float64
	Submit       SubmitConfig
	ConfigPath   string
}

// defaults returns a Config populated with built-in defaults.
func defaults() *Config {
	return &Config{
		URL:          DefaultURL,
		OutputFormat: DefaultOutputFormat,
		Color:        DefaultColor,
		Timeout:      DefaultTimeout,
		Retries:      DefaultRetries,
		RetryDelay:   DefaultRetryDelay,
		Submit:       SubmitConfig{SyslogFacility: "user"},
	}
}

// searchPaths returns the config file locations in precedence order.
func searchPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"./statshed.yaml",
		filepath.Join(home, ".config", "statshed", "statshed.yaml"),
		"/etc/statshed/statshed.yaml",
	}
}

// FromSources builds a Config by merging all sources.
//
// configPath is an explicit path (CLI arg or env var, may be empty). cliURL,
// when non-empty, overrides the URL. cliNoColor forces color off and cliJSON
// forces JSON output — both highest precedence.
func FromSources(configPath, cliURL string, cliNoColor, cliJSON bool) (*Config, error) {
	cfg := defaults()

	filePath, err := findConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	if filePath != "" {
		cfg, err = loadConfigFile(filePath)
		if err != nil {
			return nil, err
		}
		cfg.ConfigPath = filePath
	}

	if envURL := os.Getenv("STATSHED_URL"); envURL != "" {
		cfg.URL = envURL
	}

	if cliURL != "" {
		cfg.URL = cliURL
	}
	if cliNoColor {
		cfg.Color = "never"
	}
	if cliJSON {
		cfg.OutputFormat = "json"
	}

	return cfg, nil
}

// findConfigFile locates the config file, honoring an explicit path then the
// STATSHED_CONFIG env var then the default search paths.
func findConfigFile(explicit string) (string, error) {
	if explicit != "" {
		if !fileExists(explicit) {
			return "", errors.Config("Configuration file not found: %s", explicit)
		}
		return explicit, nil
	}

	if env := os.Getenv("STATSHED_CONFIG"); env != "" {
		if !fileExists(env) {
			return "", errors.Config("Configuration file not found: %s", env)
		}
		return env, nil
	}

	for _, p := range searchPaths() {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// rawConfig mirrors the YAML schema. Pointers distinguish "absent" from a zero
// value so we only override defaults for keys actually present. Color uses a
// value yaml.Node (its Kind is 0 when absent) so a scalar bool or string is
// captured reliably for custom validation.
type rawConfig struct {
	URL          *string    `yaml:"url"`
	OutputFormat *string    `yaml:"output_format"`
	Color        yaml.Node  `yaml:"color"`
	Timeout      *int       `yaml:"timeout"`
	Retries      *int       `yaml:"retries"`
	RetryDelay   *float64   `yaml:"retry_delay"`
	Submit       *rawSubmit `yaml:"submit"`
}

type rawSubmit struct {
	Syslog         *bool   `yaml:"syslog"`
	SyslogFacility *string `yaml:"syslog_facility"`
	Strict         *bool   `yaml:"strict"`
}

func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Config("Cannot read config file %s: %v", path, err)
	}
	if len(data) == 0 {
		return defaults(), nil
	}

	// AIDEV-NOTE: Decode into a generic node first so we can reject a
	// top-level scalar/sequence with the same message the Python CLI used.
	var top yaml.Node
	if err := yaml.Unmarshal(data, &top); err != nil {
		return nil, errors.Config("Invalid YAML in config file %s: %v", path, err)
	}
	if top.Kind == 0 { // empty document
		return defaults(), nil
	}
	content := &top
	if top.Kind == yaml.DocumentNode {
		if len(top.Content) == 0 {
			return defaults(), nil
		}
		content = top.Content[0]
	}
	if content.Kind != yaml.MappingNode {
		return nil, errors.Config("Config file must contain a YAML mapping in %s", path)
	}

	var raw rawConfig
	if err := content.Decode(&raw); err != nil {
		return nil, errors.Config("Invalid YAML in config file %s: %v", path, err)
	}
	return parseConfig(&raw, path)
}

func parseConfig(raw *rawConfig, path string) (*Config, error) {
	cfg := defaults()

	if raw.URL != nil {
		cfg.URL = *raw.URL
	}

	if raw.OutputFormat != nil {
		if *raw.OutputFormat != "table" && *raw.OutputFormat != "json" {
			return nil, errors.Config(
				"Config 'output_format' must be 'table' or 'json' in %s, got '%s'",
				path, *raw.OutputFormat)
		}
		cfg.OutputFormat = *raw.OutputFormat
	}

	// Kind == 0 means the key was absent.
	if raw.Color.Kind != 0 {
		color, err := parseColor(&raw.Color, path)
		if err != nil {
			return nil, err
		}
		cfg.Color = color
	}

	if raw.Timeout != nil {
		if *raw.Timeout <= 0 {
			return nil, errors.Config("Config 'timeout' must be a positive integer in %s", path)
		}
		cfg.Timeout = *raw.Timeout
	}

	if raw.Retries != nil {
		if *raw.Retries < 0 {
			return nil, errors.Config("Config 'retries' must be a non-negative integer in %s", path)
		}
		cfg.Retries = *raw.Retries
	}

	if raw.RetryDelay != nil {
		if *raw.RetryDelay < 0 {
			return nil, errors.Config("Config 'retry_delay' must be a non-negative number in %s", path)
		}
		cfg.RetryDelay = *raw.RetryDelay
	}

	if raw.Submit != nil {
		if raw.Submit.Syslog != nil {
			cfg.Submit.Syslog = *raw.Submit.Syslog
		}
		if raw.Submit.SyslogFacility != nil {
			cfg.Submit.SyslogFacility = *raw.Submit.SyslogFacility
		}
		if raw.Submit.Strict != nil {
			cfg.Submit.Strict = *raw.Submit.Strict
		}
	}

	return cfg, nil
}

// parseColor accepts a string ("auto"/"always"/"never") or a bool
// (true=always, false=never), mirroring the Python config semantics.
func parseColor(node *yaml.Node, path string) (string, error) {
	if node.Kind == yaml.ScalarNode {
		switch node.Tag {
		case "!!bool":
			var b bool
			if err := node.Decode(&b); err == nil {
				if b {
					return "always", nil
				}
				return "never", nil
			}
		case "!!str":
			var s string
			if err := node.Decode(&s); err == nil {
				if s == "auto" || s == "always" || s == "never" {
					return s, nil
				}
			}
		}
	}
	return "", errors.Config(
		"Config 'color' must be true, false, or 'auto' in %s, got '%s'", path, node.Value)
}
