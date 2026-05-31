# Changelog

All notable changes to the StatShed CLI (Go) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.2] - 2026-05-31

Initial Go release. This is a from-scratch Go port of the Python `statshed-cli`,
preserving its commands, flags, configuration, and exit codes while shipping as
a single dependency-free binary.

### Added

- **Commands**: `submit`, `stream`, `wrap`, `health`, `groups`, `jobs`,
  `config`, `group-config`, and `completion`.
- `submit --log` to attach a log file via multipart/form-data.
- `stream` — debounced, filtered submission of stdin lines as progress updates.
- `wrap` — run a command, forward IO, tee output, and submit progress, with
  `--report-exit`, `--suppress-exitcode`, and `--attach-log`.
- YAML configuration with discovery and full precedence (defaults < file < env
  < CLI), matching the Python CLI.
- Plain, colored (ANSI), and JSON output formatters with TTY auto-detection.
- Retry with exponential backoff and jitter for transient failures.
- Lenient-by-default error handling for `submit`/`stream`/`wrap` (safe under
  `set -e`), with `--strict` to propagate errors, plus optional syslog logging.
- Dynamic shell completion (group/job names) for bash, zsh, fish, and
  powershell via cobra.
- Packaging for Debian, RPM, and Nix; man page and shell completions installed
  by each package.

### Notes

- Output layout matches the Python *plain* formatter exactly. The Python *Rich*
  tables are replaced by an ANSI color formatter that uses the same textual
  layout.
