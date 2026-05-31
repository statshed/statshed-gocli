# StatShed CLI (Go)

Command-line interface for the StatShed status dashboard — a single static Go
binary named `statshed`.

This is a Go port of the original Python `statshed-cli`. It is behaviour- and
exit-code-compatible: the same commands, flags, configuration file, environment
variables, and exit codes, with no runtime dependencies.

## Installation

### From a package

Pre-built packages install the `statshed` binary, its man page, and shell
completions:

```bash
# Debian / Ubuntu
sudo dpkg -i statshed-cli_*.deb

# RHEL / Fedora / SUSE
sudo rpm -i statshed-cli-*.rpm

# Nix (flake)
nix profile install github:statshed/statshed-cli
```

See [`packaging/`](packaging/) and [`debian/`](debian/) for how each package is
built.

### From source

```bash
go install github.com/statshed/statshed-cli/cmd/statshed@latest
```

or, from a checkout:

```bash
make build      # produces ./statshed
make install    # installs to /usr/local (override with PREFIX=...)
```

## Usage

### Submit job status

```bash
statshed submit -g nightly-builds -j backend-tests -s success -m "All tests passed"
statshed submit -g nightly-builds -j backend-tests -s error    -m "3 tests failed"
statshed submit -g nightly-builds -j backend-tests -s progress -m "Running tests..."

# Attach a log file
statshed submit -g ci -j build -s error -m "failed" --log build.log
```

### Stream progress from stdin

```bash
# Each line becomes a debounced "progress" update (last-wins within --min-time)
tail -f deploy.log | statshed stream -g deploys -j web --min-time 30 \
    --regex 'step|done' --ignore 'DEBUG'
```

### Wrap a command

```bash
# Forward IO, submit output as progress, and report the final status
statshed wrap -g ci -j build --report-exit -- make all

# Attach captured output on failure
statshed wrap -g ci -j build --attach-log -- ./run-tests.sh
```

### Inspect and configure

```bash
statshed health
statshed groups
statshed jobs nightly-builds
statshed config
statshed config --progress-timeout 10 --staleness-timeout 48
statshed group-config nightly-builds --progress-timeout 15
statshed group-config nightly-builds --reset-progress-timeout
```

## Configuration file

The CLI reads YAML configuration from these locations (highest precedence
first):

1. Path from `--config` or `STATSHED_CONFIG`
2. `./statshed.yaml`
3. `~/.config/statshed/statshed.yaml`
4. `/etc/statshed/statshed.yaml`

```yaml
url: http://localhost:7828
output_format: table   # table | json
color: auto            # auto | always | never  (or true/false)
timeout: 10
retries: 0
retry_delay: 1.0

submit:
  syslog: false
  syslog_facility: user   # user | daemon | local0..local7
  strict: false
```

Precedence overall: built-in defaults < config file < environment variables <
command-line flags.

## Environment variables

- `STATSHED_URL` — API server URL
- `STATSHED_CONFIG` — path to a configuration file

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Health check returned unhealthy status |
| 2 | API error |
| 3 | Connection error |
| 4 | Timeout error |
| 5 | Configuration error |
| 10 | Invalid arguments |
| 11 | Resource not found |

`submit`, `stream`, and `wrap` are *lenient* by default: submission errors are
logged (to stderr, or syslog when configured) but the command still exits 0,
which is safe under `set -e`. Pass `--strict` to propagate error exit codes.

## Shell completion

```bash
statshed completion bash > /etc/bash_completion.d/statshed
statshed completion zsh  > "${fpath[1]}/_statshed"
statshed completion fish > ~/.config/fish/completions/statshed.fish
```

Completion suggests group and job names dynamically by querying the server,
falling back to no suggestions when it is unreachable.

## Project layout

```
cmd/statshed         entry point (version injected via -ldflags)
internal/cli         cobra commands and wiring
internal/client      HTTP client with retry/backoff
internal/config      YAML config discovery and precedence
internal/output      plain / JSON / ANSI-color formatters
internal/stream      debounced, filtered line processor
internal/wrap        subprocess IO multiplexing for `wrap`
internal/errors      error kinds and the exit-code scheme
debian/ packaging/ nix/   packaging for Debian, RPM, and Nix
```

## Development

```bash
make build      # build ./statshed
make test       # go test ./...
make vet        # go vet ./...
make fmt        # gofmt -w .
```

Dependencies are vendored under `vendor/` so the packaging builds are fully
offline/hermetic.

## License

CC0-1.0 (Public Domain Dedication). See [LICENSE](LICENSE).
