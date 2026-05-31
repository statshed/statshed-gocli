# Releasing StatShed CLI

Releases are automated by `.github/workflows/release.yml`. Pushing a `vX.Y.Z`
tag builds and publishes everything; you do not build packages by hand.

## What a release produces

Pushing a `vX.Y.Z` tag creates a **GitHub Release** with these assets attached:

- Static binaries (each a `.tar.gz` containing the `statshed` binary, the man
  page, and bash/zsh/fish completions) for:
  - `linux/amd64`, `linux/arm64`
  - `darwin/amd64`, `darwin/arm64`
- `.deb` packages for **Debian trixie** and **Ubuntu noble**
- An `.rpm` package for **Fedora**
- A `SHA256SUMS` file covering every asset

Release notes are taken from the matching `## [X.Y.Z]` section of
`CHANGELOG.md` (falling back to GitHub's auto-generated notes if that section
is missing).

> Windows is intentionally not built — `internal/cli/syslog.go` uses
> `log/syslog`, which Go does not support on Windows.

Separately, pushing a `vX.Y.Z` tag also makes the Go module installable via the
module proxy with no extra step:

```sh
go install github.com/statshed/statshed-cli/cmd/statshed@vX.Y.Z
```

## The version gate

Before anything builds, the workflow fails unless the tag version (`X.Y.Z`,
the tag minus its leading `v`) matches the version recorded in **all five** of
these files. Bump every one of them in the same commit:

| File | What to change |
|------|----------------|
| `debian/changelog` | Add a new top stanza `statshed-cli (X.Y.Z) unstable; urgency=medium` |
| `packaging/rpm/statshed-cli.spec` | `%global version X.Y.Z` (and add a `%changelog` entry) |
| `nix/package.nix` | `version = "X.Y.Z";` **and** the `main.version=X.Y.Z` in `ldflags` |
| `Makefile` | `VERSION ?= X.Y.Z` |
| `CHANGELOG.md` | Add a new `## [X.Y.Z] - YYYY-MM-DD` section at the top |

If any file disagrees, the `check-version` job fails fast and nothing is built.

## Step-by-step

The example below releases **1.0.3**.

### 1. Bump the version in all five files

Edit each file from the table above. For `debian/changelog`, the easiest way is
`dch` (from `devscripts`):

```sh
dch --newversion 1.0.3 --distribution unstable "Describe the changes."
dch --release ""    # finalize the timestamp/author line
```

…or add the stanza by hand:

```
statshed-cli (1.0.3) unstable; urgency=medium

  * Describe the changes.

 -- Sean <jafo00+oss@gmail.com>  Mon, 02 Jun 2026 09:00:00 -0600
```

For `CHANGELOG.md`, add a new section that describes the release (this becomes
the GitHub Release notes):

```markdown
## [1.0.3] - 2026-06-02

### Fixed
- ...
```

### 2. Sanity-check locally

```sh
make build        # confirms it compiles
make test         # go test ./...
gofmt -l . | grep -v '^vendor/'   # should print nothing
```

### 3. Commit the version bump

```sh
git add debian/changelog packaging/rpm/statshed-cli.spec nix/package.nix \
        Makefile CHANGELOG.md
git commit -m "Release 1.0.3"
git push origin main
```

### 4. (Recommended) Dry-run the pipeline

In the GitHub UI: **Actions → Release → Run workflow**. This runs the
`workflow_dispatch` path: it builds the binaries, `.deb`, and `.rpm` and uploads
them as workflow artifacts, **without** creating a Release. Use it to confirm
the package builds succeed — especially the first time, since the `.deb`/`.rpm`
container builds can't be exercised locally.

### 5. Tag and push

The tag must be `v` + the exact version from the five files.

```sh
git tag v1.0.3
git push origin v1.0.3
```

That triggers the full release. Watch it under the **Actions** tab; when it
finishes, the GitHub Release appears under **Releases** with all assets attached.

## Troubleshooting

- **`check-version` failed: "<file> version '…' != release version '…'"** — one
  of the five files wasn't bumped (or the tag doesn't match them). Fix the file
  or re-tag. To move a tag you pushed by mistake:
  ```sh
  git tag -d v1.0.3 && git push origin :refs/tags/v1.0.3   # delete local + remote
  # fix the files, commit, then re-tag and push
  ```
- **A `.deb`/`.rpm` job failed on the Go toolchain** — `go.mod` pins the Go
  version and the packaging sets `GOTOOLCHAIN=local`; the container jobs install
  that toolchain via `setup-go`. If `go.mod`'s version changes, no action is
  needed (the jobs read it), but a brand-new Go version must be available to
  `setup-go`.
- **`lintian`/`autopkgtest` warnings** — these are reported but **non-fatal**;
  they never block a release. Review them in the job logs and tighten the steps
  in `release.yml` if you want them enforced.

## Re-running a release

`gh release create` (used by the workflow) fails if the Release already exists.
If you need to rebuild the same version, delete the existing GitHub Release
first, then re-push the tag — or cut a new patch version.
