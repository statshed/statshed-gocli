#!/bin/sh
# Build the RPM from a source tarball using packaging/rpm/statshed-cli.spec.
#
# Usage: packaging/build-rpm.sh [VERSION]
# Requires: rpm-build, golang. The source tarball includes vendored modules so
# the build is offline.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-1.0.2}"
NAME="statshed-cli"

if ! command -v rpmbuild >/dev/null 2>&1; then
	echo "error: rpmbuild not found (install rpm-build)" >&2
	exit 1
fi

TOPDIR="$(mktemp -d)"
trap 'rm -rf "$TOPDIR"' EXIT
mkdir -p "$TOPDIR/SOURCES" "$TOPDIR/SPECS"

# AIDEV-NOTE: Include vendor/ in the tarball so rpmbuild builds without network.
TARBALL="$TOPDIR/SOURCES/${NAME}-${VERSION}.tar.gz"
if command -v git >/dev/null 2>&1 && git rev-parse --git-dir >/dev/null 2>&1; then
	git archive --format=tar --prefix="${NAME}-${VERSION}/" HEAD >"$TOPDIR/base.tar"
	gzip -c "$TOPDIR/base.tar" >"$TARBALL"
else
	tar --exclude-vcs --transform "s,^\.,${NAME}-${VERSION}," -czf "$TARBALL" .
fi

cp packaging/rpm/${NAME}.spec "$TOPDIR/SPECS/"
rpmbuild --define "_topdir $TOPDIR" --define "version $VERSION" \
	-bb "$TOPDIR/SPECS/${NAME}.spec"

mkdir -p "$ROOT/dist"
find "$TOPDIR/RPMS" -name '*.rpm' -exec cp {} "$ROOT/dist/" \;
echo "Built packages:"
ls -1 "$ROOT"/dist/*.rpm 2>/dev/null || true
