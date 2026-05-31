# GitHub Actions: CI + tag-triggered release pipeline

Date: 2026-05-31
Status: Approved (brainstorming) — implementation in progress

## Goal

Add GitHub Actions that, when a `vX.Y.Z` tag is pushed, build and publish all
distributable artifacts for the StatShed Go CLI and attach them to a GitHub
Release. Add a companion CI workflow that runs tests on pushes/PRs.

## Decisions (from brainstorming)

- **Trigger:** push a `v*` tag. The workflow builds everything AND creates the
  GitHub Release (one command to ship: `git tag vX.Y.Z && git push --tags`).
- **Artifacts:** cross-platform static binaries + `.deb` (Debian/Ubuntu) +
  `.rpm` (Fedora). No Docker image.
  - **Windows dropped during implementation:** `internal/cli/syslog.go` imports
    `log/syslog`, which Go does not implement on Windows, so windows targets fail
    to compile. Binary targets are therefore `linux/{amd64,arm64}` and
    `darwin/{amd64,arm64}` (4 targets, all verified to cross-build locally).
    Adding Windows would require a build-tagged syslog stub (a code change).
- **CI:** a separate `ci.yml` runs `go test`, `go vet`, and a `gofmt` check on
  push/PR to `main`/`master`.
- **Approach:** one hand-rolled unified `release.yml` (not GoReleaser, not split
  per-format workflows). Reuses the existing `debian/` metadata and rpm spec
  unchanged; keeps the project's "vendored, offline, dependency-free" ethos.

## Files

- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `.github/dependabot.yml` — weekly `github-actions` updates only (keeps the
  SHA pins current). **Not** `gomod`: `vendor/` is committed, and a gomod bump
  would leave `vendor/` stale and break the offline build.

## `release.yml` job graph

```
check-version ─┬─> build-binaries (matrix: 6 targets) ─┐
               ├─> build-deb (matrix: debian, ubuntu)   ├─> release
               └─> build-rpm (fedora)                   ┘   (push only)
```

### check-version

The pushed tag `vX.Y.Z` is authoritative. On `workflow_dispatch` (dry-run), the
version is read from `debian/changelog` instead. The job fails unless that
version matches the version recorded in **all** of:

- `debian/changelog`
- `packaging/rpm/statshed-cli.spec` (`%global version`)
- `nix/package.nix` (`version = "..."`)
- `Makefile` (`VERSION ?=`)
- `CHANGELOG.md` (top `## [X.Y.Z]` heading)

(The `Makefile` AIDEV-NOTE already requires these to stay in sync; this enforces
it.) Exposes `version` as a job output for downstream jobs.

### build-binaries

Matrix of 4 targets: `linux/{amd64,arm64}`, `darwin/{amd64,arm64}` (Windows
excluded — see Decisions). Each job:

- `CGO_ENABLED=0 GOFLAGS=-mod=vendor go build -trimpath -ldflags "-s -w -X main.version=<version>"`
  cross-compiled for the target (pure Go → trivial cross-compile from Linux).
- Generates bash/zsh/fish completions **host-side** with
  `go run ./cmd/statshed completion …` (the foreign target binary can't run on
  the Linux runner; completions are platform-independent text).
- Assembles a `statshed-cli-<version>-<os>-<arch>.tar.gz` containing: the
  `statshed` binary, `LICENSE`, `README.md`, `CHANGELOG.md`, `docs/statshed.1`,
  and the completions.

### build-deb

Containers `debian:trixie` and `ubuntu:noble`. Installs build tooling, installs
`Build-Depends` via `mk-build-deps` from `debian/control`, then
`dpkg-buildpackage -us -uc -b` (the build runs `go test` via
`override_dh_auto_test`). `lintian --fail-on error` and `autopkgtest … -- null`
run **non-fatal** (reported, never block a release — mirrors `pycli/release.yml`;
can be tightened once validated). Artifacts renamed with a `_<distro>-<release>`
suffix.

### build-rpm

Container `fedora:latest`. Builds a vendored source tarball with `git archive`,
then `rpmbuild -bb --define "version <version>"` against the existing spec (its
`%check` runs `go test`). Output collected from `~/rpmbuild/RPMS/`.

### release

`if: github.event_name == 'push'` (skipped on dry-run). Downloads all artifacts,
generates `SHA256SUMS` over them, extracts the matching `## [X.Y.Z]` section from
`CHANGELOG.md` for the release notes (falls back to `--generate-notes`), and runs
`gh release create "<tag>"` attaching every artifact.

## Toolchain handling (critical detail)

`go.mod` pins `go 1.26.2`, and both `debian/rules` and the rpm spec set
`GOTOOLCHAIN=local`. A container's distro Go (older than 1.26.2) would therefore
make the deb/rpm build **fail** with a toolchain error. These packaging paths
have never actually been executed (per the project handoff), so this is latent.

Fix entirely within the workflow (no packaging changes): the deb and rpm
container jobs run `actions/setup-go` with `go-version-file: go.mod`, which puts
the exact `go.mod` Go version first on `PATH`; the existing `GOTOOLCHAIN=local`
build then uses it. The distro `golang`/`golang-go` is still installed to satisfy
`Build-Depends`/`BuildRequires`, but the setup-go toolchain wins on `PATH`.

If the user later prefers to fix the root cause (relax `go.mod` and/or
`GOTOOLCHAIN`), that is a packaging change handled separately.

## Security / supply chain

- Default `permissions: contents: read`; only the `release` job gets
  `contents: write`. No secrets — `GITHUB_TOKEN` covers the Release.
- All actions pinned to commit SHAs with trailing version comments:
  - `actions/checkout` v4.3.1 `34e114876b0b11c390a56381ad16ebd13914f8d5`
  - `actions/setup-go` v6.4.0 `4a3601121dd01d1626a1e23e37211e3254c1c06c`
  - `actions/upload-artifact` v4.6.2 `ea165f8d65b6e75b540449e92b4886f43607fa02`
  - `actions/download-artifact` v4.3.0 `d3f86a106a0bac45b974a628896c90dbdf5c8093`
- `concurrency` guards; `fail-fast: false` on matrices.

## Dry-run

`workflow_dispatch` on `release.yml` runs check-version + all three build jobs
(uploading artifacts as workflow artifacts) but skips Release creation — lets you
validate the whole pipeline before committing to a real tag.

## Implementation outcome (2026-05-31)

Verified locally:

- All 4 binary targets cross-compile; host-side `go run … completion` and the
  `tar.gz` assembly work. Windows confirmed unbuildable (`log/syslog`).
- Version-gate `sed` extractions return `1.0.2` for all 5 files; the CHANGELOG
  `awk` notes extraction works. `vendor/` is committed (rpm tarball is offline).
- `go vet ./...` and `go test ./...` pass.

Two fixes made during implementation:

- **`ci.yml` gofmt:** `gofmt -l .` flags vendored third-party files (not all
  gofmt-clean) and would fail CI. Scoped it to exclude `vendor/`.
- **`build-rpm`:** added `ca-certificates` to the Fedora `dnf install` so the
  `git` checkout / setup-go download have a CA bundle in the minimal image.

A four-reviewer adversarial workflow (Actions semantics, shell correctness,
security, packaging/toolchain) read the files plus `debian/rules`/spec/`go.mod`
and reported no further issues.

Cannot be exercised off GitHub: the `.deb`/`.rpm` container builds (no
`dpkg-buildpackage`/`rpmbuild` on the dev host) and the setup-go-on-PATH +
`GOTOOLCHAIN=local` interaction inside containers. Validate these with a
`workflow_dispatch` dry-run before the first real tag.
