#!/bin/sh
# Installs the Kaiten CLI from its GitHub releases on Linux and macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.sh | sh -s -- 1.2.3
#
# Environment:
#   KAITEN_VERSION       version to install, with or without the leading v (default: latest release)
#   KAITEN_INSTALL_DIR   directory to install into (default: /usr/local/bin, with sudo when needed)
#   KAITEN_DOWNLOAD_URL  base URL serving the release assets, for mirrors (default: the GitHub release)
#
# The archive's SHA-256 is verified against the release's checksums.txt before anything is installed.
set -eu

REPO="kaitencloud/cli"
BINARY="kaiten"
INSTALL_DIR="${KAITEN_INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-${KAITEN_VERSION:-}}"

say() { printf '%s\n' "$*"; }
fail() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

fetch() {
	if have curl; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif have wget; then
		wget -q -O "$2" "$1"
	else
		fail "curl or wget is required"
	fi
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	linux | darwin) ;;
	mingw* | msys* | cygwin*) fail "on Windows, run: powershell -c \"irm https://raw.githubusercontent.com/$REPO/main/install/install.ps1 | iex\"" ;;
	*) fail "unsupported operating system: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported architecture: $arch (releases ship amd64 and arm64)" ;;
esac

asset="${BINARY}_${os}_${arch}.tar.gz"
if [ -n "${KAITEN_DOWNLOAD_URL:-}" ]; then
	base="${KAITEN_DOWNLOAD_URL%/}"
elif [ -n "$VERSION" ]; then
	case "$VERSION" in v*) ;; *) VERSION="v$VERSION" ;; esac
	base="https://github.com/$REPO/releases/download/$VERSION"
else
	base="https://github.com/$REPO/releases/latest/download"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "Downloading $base/$asset"
fetch "$base/$asset" "$tmp/$asset"
fetch "$base/checksums.txt" "$tmp/checksums.txt"

expected=$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "checksums.txt has no entry for $asset"
if have sha256sum; then
	actual=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
elif have shasum; then
	actual=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is required to verify the download"
fi
[ "$actual" = "$expected" ] || fail "checksum mismatch for $asset: expected $expected, got $actual"

tar -xzf "$tmp/$asset" -C "$tmp" "$BINARY"

as_root() {
	if [ -n "$use_sudo" ]; then sudo "$@"; else "$@"; fi
}

# Writability is decided by the closest directory that already exists, so a
# missing ~/.local/bin is created by the user and a missing /opt/x by root.
probe="$INSTALL_DIR"
while [ ! -d "$probe" ]; do probe=$(dirname "$probe"); done
use_sudo=""
if [ ! -w "$probe" ]; then
	have sudo || fail "cannot write to $INSTALL_DIR; set KAITEN_INSTALL_DIR to a writable directory"
	say "Installing into $INSTALL_DIR needs sudo."
	use_sudo=1
fi
as_root mkdir -p "$INSTALL_DIR"
as_root install -m 755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"

say "Installed $("$INSTALL_DIR/$BINARY" version) to $INSTALL_DIR/$BINARY"
case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) say "Note: $INSTALL_DIR is not in your PATH." ;;
esac
say "Run 'kaiten config set base-url <url>' and 'kaiten doctor' to get started."
