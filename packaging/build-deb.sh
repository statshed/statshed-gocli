#!/bin/sh
# Build the Debian package from the repo root using the debian/ metadata.
#
# Usage: packaging/build-deb.sh [VERSION]
# Requires: dpkg-dev, debhelper, golang-go. The .deb is written to ../.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v dpkg-buildpackage >/dev/null 2>&1; then
	echo "error: dpkg-buildpackage not found (install dpkg-dev, debhelper)" >&2
	exit 1
fi

# Binary-only build, unsigned. The package version comes from debian/changelog.
dpkg-buildpackage -us -uc -b

echo "Built packages:"
ls -1 ../statshed-cli_*.deb 2>/dev/null || true
