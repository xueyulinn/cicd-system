#!/usr/bin/env bash
# Install cicd CLI from GitHub Release assets.
# Usage: curl -sSL https://raw.githubusercontent.com/CS7580-SEA-SP26/e-team/<branch>/scripts/install.sh | sh
#    or: ./install.sh [VERSION]   e.g. ./install.sh v0.1.0
# Without VERSION, uses latest release tag from GitHub API.

set -e

REPO="CS7580-SEA-SP26/e-team"
if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  echo "Usage: $0 [VERSION]"
  echo "  Install cicd CLI from GitHub Release. VERSION defaults to latest (e.g. v0.1.0)."
  echo "  Example: curl -sSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sh"
  echo "           $0 v0.1.0"
  exit 0
fi
BINARY_NAME="cicd"
# Install directory (override with CICD_BIN or PREFIX/bin)
DEST_DIR="${CICD_BIN:-${PREFIX:-$HOME}/bin}"

# Resolve VERSION: first arg, or env CICD_VERSION, or fetch latest
VERSION="${1:-$CICD_VERSION}"
if [ -z "$VERSION" ]; then
  if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
  fi
  if [ -z "$VERSION" ]; then
    echo "Error: Could not determine version. Pass it as first argument: install.sh v0.1.0" >&2
    exit 1
  fi
fi

# Normalize version (strip leading v if present for URL)
TAG="$VERSION"
[ "${TAG#v}" != "$TAG" ] || TAG="v${TAG}"

# Detect OS and arch (Linux/Darwin, amd64/arm64)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac
case "$OS" in
  linux|darwin) ;;
  *)
    echo "Error: Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

ARTIFACT="${BINARY_NAME}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARTIFACT}"

mkdir -p "$DEST_DIR"
if command -v curl >/dev/null 2>&1; then
  echo "Downloading ${ARTIFACT} from ${URL} ..."
  curl -sSLf -o "${DEST_DIR}/${BINARY_NAME}.tmp" "$URL"
elif command -v wget >/dev/null 2>&1; then
  echo "Downloading ${ARTIFACT} from ${URL} ..."
  wget -q -O "${DEST_DIR}/${BINARY_NAME}.tmp" "$URL"
else
  echo "Error: Need curl or wget to download the binary." >&2
  exit 1
fi

chmod +x "${DEST_DIR}/${BINARY_NAME}.tmp"
mv "${DEST_DIR}/${BINARY_NAME}.tmp" "${DEST_DIR}/${BINARY_NAME}"
echo "Installed: ${DEST_DIR}/${BINARY_NAME}"

if ! command -v "$BINARY_NAME" >/dev/null 2>&1; then
  echo "Add to PATH to run \`${BINARY_NAME}\` from the shell:"
  echo "  export PATH=\"\$PATH:${DEST_DIR}\""
  echo "You can add the above line to your shell profile (e.g. .bashrc, .zshrc)."
fi
