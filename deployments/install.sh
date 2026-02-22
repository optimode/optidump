#!/usr/bin/env bash
set -euo pipefail

REPO="optimode/optidump"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Determine version (latest release or user-specified)
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version" >&2
    exit 1
  fi
fi

BINARY="optidump-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums-sha256.txt"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading optidump ${VERSION} (${OS}/${ARCH})..."
curl -fsSL -o "${TMPDIR}/${BINARY}" "$DOWNLOAD_URL"
curl -fsSL -o "${TMPDIR}/checksums-sha256.txt" "$CHECKSUMS_URL"

# Verify checksum
echo "Verifying checksum..."
EXPECTED="$(grep "${BINARY}" "${TMPDIR}/checksums-sha256.txt" | awk '{print $1}')"
if [ -z "$EXPECTED" ]; then
  echo "Error: binary not found in checksums file" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMPDIR}/${BINARY}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${TMPDIR}/${BINARY}" | awk '{print $1}')"
else
  echo "Warning: no sha256 tool found, skipping checksum verification" >&2
  ACTUAL="$EXPECTED"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "Error: checksum mismatch" >&2
  echo "  expected: $EXPECTED" >&2
  echo "  actual:   $ACTUAL" >&2
  exit 1
fi

# Install binary
echo "Installing..."
chmod +x "${TMPDIR}/${BINARY}"
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/optidump"
  SUDO=""
else
  sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/optidump"
  SUDO="sudo"
fi

# Create directories
$SUDO mkdir -p /etc/optidump
$SUDO mkdir -p /var/log/optidump

# Download example config
CONFIG_URL="https://raw.githubusercontent.com/${REPO}/${VERSION}/configs/config.example.yml"
curl -fsSL -o "${TMPDIR}/config.example.yml" "$CONFIG_URL"

if [ ! -f /etc/optidump/config.yml ]; then
  # Fresh install: use as default config
  $SUDO cp "${TMPDIR}/config.example.yml" /etc/optidump/config.yml
  echo "Configuration: /etc/optidump/config.yml (new)"
else
  # Upgrade: keep existing config, save example for reference
  $SUDO cp "${TMPDIR}/config.example.yml" /etc/optidump/config.example.yml
  echo "Configuration: /etc/optidump/config.yml (kept existing, example saved)"
fi

echo "Installed optidump ${VERSION} to ${INSTALL_DIR}/optidump"
"${INSTALL_DIR}/optidump" --version
