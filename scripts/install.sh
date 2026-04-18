#!/usr/bin/env bash
# Install cicd CLI from GitHub Release assets.
# Usage: curl -sSL https://raw.githubusercontent.com/xueyulinn/cicd-system/<branch>/scripts/install.sh | sh
#    or: ./install.sh [--with-services] [--stack-dir DIR] [VERSION]
# Without VERSION, uses latest release tag from GitHub API.

set -e

REPO="xueyulinn/cicd-system"
BINARY_NAME="cicd"
# Install directory (override with CICD_BIN or PREFIX/bin)
DEST_DIR="${CICD_BIN:-${PREFIX:-$HOME}/bin}"
STACK_DIR="${CICD_STACK_DIR:-${CICD_HOME:-$HOME/.cicd}/stack}"
WITH_SERVICES=0
POSITIONAL_VERSION=""

usage() {
  echo "Usage: $0 [--with-services] [--stack-dir DIR] [VERSION]"
  echo "  Install cicd CLI from GitHub Release. VERSION defaults to latest (e.g. v0.1.0)."
  echo "  For private repositories, set GITHUB_TOKEN with repo read access."
  echo "Options:"
  echo "  --with-services   Also download and install compose bundle for this release."
  echo "  --stack-dir DIR   Install compose bundle into DIR (default: ${STACK_DIR})."
  echo "  -h, --help        Show help."
  echo "Examples:"
  echo "  curl -sSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sh"
  echo "  $0 v0.1.0"
  echo "  $0 --with-services v0.1.0-beta"
}

download_file() {
  src_url="$1"
  dst_path="$2"
  if command -v curl >/dev/null 2>&1; then
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      curl -sSLf \
        -H "Authorization: Bearer ${GITHUB_TOKEN}" \
        -H "Accept: application/octet-stream" \
        -o "$dst_path" "$src_url"
    else
      curl -sSLf -o "$dst_path" "$src_url"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      wget -q \
        --header="Authorization: Bearer ${GITHUB_TOKEN}" \
        --header="Accept: application/octet-stream" \
        -O "$dst_path" "$src_url"
    else
      wget -q -O "$dst_path" "$src_url"
    fi
  else
    echo "Error: Need curl or wget to download artifacts." >&2
    exit 1
  fi
}

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --with-services)
      WITH_SERVICES=1
      ;;
    --stack-dir)
      shift
      if [ -z "${1:-}" ]; then
        echo "Error: --stack-dir requires a directory path." >&2
        exit 1
      fi
      STACK_DIR="$1"
      ;;
    --*)
      echo "Error: Unknown option: $1" >&2
      usage
      exit 1
      ;;
    *)
      if [ -n "$POSITIONAL_VERSION" ]; then
        echo "Error: Multiple VERSION values provided: '$POSITIONAL_VERSION' and '$1'." >&2
        usage
        exit 1
      fi
      POSITIONAL_VERSION="$1"
      ;;
  esac
  shift
done

# Resolve VERSION: first arg, or env CICD_VERSION, or fetch latest
VERSION="${POSITIONAL_VERSION:-$CICD_VERSION}"
if [ -z "$VERSION" ]; then
  if command -v curl >/dev/null 2>&1; then
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      VERSION=$(
        curl -sSL \
          -H "Authorization: Bearer ${GITHUB_TOKEN}" \
          -H "Accept: application/vnd.github+json" \
          "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p'
      )
    else
      VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
    fi
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
echo "Downloading ${ARTIFACT} from ${URL} ..."
download_file "$URL" "${DEST_DIR}/${BINARY_NAME}.tmp"

chmod +x "${DEST_DIR}/${BINARY_NAME}.tmp"
mv "${DEST_DIR}/${BINARY_NAME}.tmp" "${DEST_DIR}/${BINARY_NAME}"
echo "Installed: ${DEST_DIR}/${BINARY_NAME}"

if ! command -v "$BINARY_NAME" >/dev/null 2>&1; then
  echo "Add to PATH to run \`${BINARY_NAME}\` from the shell:"
  echo "  export PATH=\"\$PATH:${DEST_DIR}\""
  echo "You can add the above line to your shell profile (e.g. .bashrc, .zshrc)."
fi

if [ "$WITH_SERVICES" = "1" ]; then
  BUNDLE_NAME="compose-bundle-${TAG}.tar.gz"
  BUNDLE_URL="https://github.com/${REPO}/releases/download/${TAG}/${BUNDLE_NAME}"
  TMP_DIR=$(mktemp -d)
  BUNDLE_ARCHIVE="${TMP_DIR}/${BUNDLE_NAME}"
  EXTRACT_DIR="${TMP_DIR}/extract"

  cleanup() {
    rm -rf "$TMP_DIR"
  }
  trap cleanup EXIT

  echo "Downloading ${BUNDLE_NAME} from ${BUNDLE_URL} ..."
  download_file "$BUNDLE_URL" "$BUNDLE_ARCHIVE"

  mkdir -p "$EXTRACT_DIR"
  tar -xzf "$BUNDLE_ARCHIVE" -C "$EXTRACT_DIR"

  SRC_DIR="$EXTRACT_DIR"
  shopt -s nullglob
  EXTRACT_ENTRIES=("$EXTRACT_DIR"/*)
  shopt -u nullglob
  if [ "${#EXTRACT_ENTRIES[@]}" -eq 1 ] && [ -d "${EXTRACT_ENTRIES[0]}" ]; then
    SRC_DIR="${EXTRACT_ENTRIES[0]}"
  fi

  mkdir -p "$STACK_DIR"
  cp -R "$SRC_DIR"/. "$STACK_DIR"/

  echo "Installed compose bundle to: ${STACK_DIR}"
  echo "Next steps:"
  echo "  cp \"${STACK_DIR}/.env.example\" \"${STACK_DIR}/.env\""
  echo "  docker compose -f \"${STACK_DIR}/docker-compose.yaml\" up -d"
fi
