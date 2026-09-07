#!/bin/sh
set -eu

REPO="ducva/tofu-diff"
DEFAULT_INSTALL_DIR="$HOME/.local/bin"
INSTALL_DIR="${INSTALL_DIR:-${BIN_DIR:-$DEFAULT_INSTALL_DIR}}"
VERSION="${VERSION:-""}"
VERIFY_CHECKSUM=1
BUILD_FROM_SOURCE=0

print_usage() {
  cat <<EOF
tofu-diff installer

Usage:
  install.sh [options]

Options:
  -d, --dir <DIR>        Installation directory (default: ~/.local/bin)
  -b, --bindir <DIR>     Alias for --dir
  -v, --version <VER>    Version to install (default: latest release)
  --build                Build from local source with Go instead of downloading binary
  --no-checksum          Skip SHA-256 checksum verification
  -h, --help             Show this help message

Environment Variables:
  INSTALL_DIR, BIN_DIR   Installation directory
  VERSION                Version tag to install (e.g. v0.0.11 or 0.0.11)
  GITHUB_TOKEN           GitHub personal access token (optional, for rate-limited API calls)

Examples:
  ./install.sh
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh -s -- --dir /usr/local/bin
EOF
}

# Parse command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    -d|--dir|-b|--bindir)
      if [ -z "${2:-}" ]; then
        echo "Error: $1 requires a directory argument" >&2
        exit 1
      fi
      INSTALL_DIR="$2"
      shift 2
      ;;
    --dir=*|--bindir=*)
      INSTALL_DIR="${1#*=}"
      shift
      ;;
    -v|--version)
      if [ -z "${2:-}" ]; then
        echo "Error: $1 requires a version argument" >&2
        exit 1
      fi
      VERSION="$2"
      shift 2
      ;;
    --version=*)
      VERSION="${1#*=}"
      shift
      ;;
    --build)
      BUILD_FROM_SOURCE=1
      shift
      ;;
    --no-checksum)
      VERIFY_CHECKSUM=0
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Error: unrecognized option '$1'" >&2
      print_usage >&2
      exit 1
      ;;
  esac
done

detect_os() {
  case "$(uname -s)" in
    Linux*)     echo "linux" ;;
    Darwin*)    echo "darwin" ;;
    CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
    *)          echo "" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)             echo "" ;;
  esac
}

download_file() {
  url="$1"
  dest="$2"
  auth_header=""
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    auth_header="Authorization: Bearer ${GITHUB_TOKEN}"
  fi

  if command -v curl >/dev/null 2>&1; then
    if [ -n "$auth_header" ]; then
      curl -fsSL -H "$auth_header" "$url" -o "$dest"
    else
      curl -fsSL "$url" -o "$dest"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "$auth_header" ]; then
      wget -q --header="$auth_header" -O "$dest" "$url"
    else
      wget -q -O "$dest" "$url"
    fi
  else
    echo "Error: neither curl nor wget was found. Please install curl or wget." >&2
    exit 1
  fi
}

resolve_latest_version() {
  tag=""
  # 1. First try redirect header from GitHub releases/latest (bypasses API rate limits)
  if command -v curl >/dev/null 2>&1; then
    tag="$(curl -fsSI "https://github.com/${REPO}/releases/latest" 2>/dev/null | awk '/^[[:space:]]*[Ll]ocation:/ { for (i=1; i<=NF; i++) if ($i ~ /^https?:\/\//) { sub(/.*\//, "", $i); print $i; exit } }' | tr -d '\r\n')"
  elif command -v wget >/dev/null 2>&1; then
    tag="$(wget --spider -S "https://github.com/${REPO}/releases/latest" 2>&1 | awk '/^[[:space:]]*[Ll]ocation:/ { for (i=1; i<=NF; i++) if ($i ~ /^https?:\/\//) { sub(/.*\//, "", $i); print $i; exit } }' | tr -d '\r\n')"
  fi

  # 2. Fallback to GitHub API
  if [ -z "$tag" ]; then
    api_url="https://api.github.com/repos/${REPO}/releases/latest"
    auth_header=""
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      auth_header="Authorization: Bearer ${GITHUB_TOKEN}"
    fi

    if command -v curl >/dev/null 2>&1; then
      if [ -n "$auth_header" ]; then
        tag="$(curl -fsSL -H "$auth_header" "$api_url" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' | tr -d '\r\n')"
      else
        tag="$(curl -fsSL "$api_url" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' | tr -d '\r\n')"
      fi
    elif command -v wget >/dev/null 2>&1; then
      if [ -n "$auth_header" ]; then
        tag="$(wget -qO- --header="$auth_header" "$api_url" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' | tr -d '\r\n')"
      else
        tag="$(wget -qO- "$api_url" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' | tr -d '\r\n')"
      fi
    fi
  fi

  if [ -z "$tag" ]; then
    echo "Error: failed to determine latest release tag for ${REPO}" >&2
    exit 1
  fi
  echo "$tag"
}

compute_sha256() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
  else
    echo ""
  fi
}

check_in_path() {
  target_dir="$1"
  case ":$PATH:" in
    *":$target_dir:"*) return 0 ;;
  esac
  if [ -n "${HOME:-}" ]; then
    tilde_dir="~${target_dir#"$HOME"}"
    case ":$PATH:" in
      *":$tilde_dir:"*) return 0 ;;
    esac
  fi
  return 1
}

# Source build flow
if [ "$BUILD_FROM_SOURCE" -eq 1 ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Error: Go compiler is required to build from source" >&2
    exit 1
  fi

  if [ -f "./cmd/tofu-diff/main.go" ] && [ -f "./go.mod" ]; then
    echo "Building tofu-diff from local source..."
    mkdir -p "$INSTALL_DIR"
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$INSTALL_DIR/tofu-diff" ./cmd/tofu-diff
    chmod 755 "$INSTALL_DIR/tofu-diff"
    echo "Successfully built and installed tofu-diff to ${INSTALL_DIR}/tofu-diff"
  else
    echo "Error: current directory does not contain tofu-diff source files" >&2
    exit 1
  fi

  if ! check_in_path "$INSTALL_DIR"; then
    echo ""
    echo "Notice: '${INSTALL_DIR}' is not in your PATH."
    echo "To run tofu-diff directly, add this directory to your PATH:"
    echo ""
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
  fi
  exit 0
fi

# Binary download flow
OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ -z "$OS" ]; then
  echo "Error: unsupported operating system '$(uname -s)'" >&2
  exit 1
fi

if [ -z "$ARCH" ]; then
  echo "Error: unsupported architecture '$(uname -m)'" >&2
  exit 1
fi

BINARY_NAME="tofu-diff"
if [ "$OS" = "windows" ]; then
  BINARY_NAME="tofu-diff.exe"
  ARCHIVE_NAME="tofu-diff_${OS}_${ARCH}.zip"
else
  ARCHIVE_NAME="tofu-diff_${OS}_${ARCH}.tar.gz"
fi

if [ -n "$VERSION" ]; then
  case "$VERSION" in
    v*) ;;
    *) VERSION="v$VERSION" ;;
  esac
else
  echo "Resolving latest tofu-diff release..."
  VERSION="$(resolve_latest_version)"
fi

echo "Selected version: ${VERSION}"
echo "Platform: ${OS}/${ARCH}"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'tofu-diff-install.XXXXXX')"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

RELEASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${RELEASE_URL}/${ARCHIVE_NAME}"
CHECKSUMS_URL="${RELEASE_URL}/checksums.txt"

echo "Downloading ${ARCHIVE_NAME}..."
download_file "$ARCHIVE_URL" "$TMP_DIR/$ARCHIVE_NAME"

if [ "$VERIFY_CHECKSUM" -eq 1 ]; then
  echo "Verifying checksum..."
  if download_file "$CHECKSUMS_URL" "$TMP_DIR/checksums.txt" 2>/dev/null; then
    EXPECTED_SHA="$(grep "[[:space:]]${ARCHIVE_NAME}\$" "$TMP_DIR/checksums.txt" | awk '{print $1}')"
    ACTUAL_SHA="$(compute_sha256 "$TMP_DIR/$ARCHIVE_NAME")"

    if [ -n "$EXPECTED_SHA" ] && [ -n "$ACTUAL_SHA" ]; then
      if [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
        echo "Error: checksum verification failed for ${ARCHIVE_NAME}" >&2
        echo "  Expected: ${EXPECTED_SHA}" >&2
        echo "  Actual:   ${ACTUAL_SHA}" >&2
        exit 1
      fi
      echo "Checksum verified."
    else
      echo "Notice: unable to compute SHA-256 checksum locally; skipping verification."
    fi
  else
    echo "Notice: checksums.txt not available for ${VERSION}; skipping checksum verification."
  fi
fi

echo "Extracting binary..."
case "$ARCHIVE_NAME" in
  *.tar.gz|*.tgz)
    tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"
    ;;
  *.zip)
    if command -v unzip >/dev/null 2>&1; then
      unzip -q -o "$TMP_DIR/$ARCHIVE_NAME" -d "$TMP_DIR"
    else
      echo "Error: unzip is required to extract Windows zip archives" >&2
      exit 1
    fi
    ;;
esac

if [ ! -f "$TMP_DIR/$BINARY_NAME" ]; then
  echo "Error: binary '${BINARY_NAME}' not found in archive" >&2
  exit 1
fi

echo "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"

# Copy to temporary file first and atomically replace to avoid ETXTBSY if binary is running
cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/.${BINARY_NAME}.tmp.$$"
chmod 755 "$INSTALL_DIR/.${BINARY_NAME}.tmp.$$"
mv -f "$INSTALL_DIR/.${BINARY_NAME}.tmp.$$" "$INSTALL_DIR/$BINARY_NAME"

echo "tofu-diff (${VERSION}) successfully installed to ${INSTALL_DIR}/${BINARY_NAME}"

if ! check_in_path "$INSTALL_DIR"; then
  echo ""
  echo "Notice: '${INSTALL_DIR}' is not in your PATH."
  echo "To run tofu-diff directly, add this directory to your PATH:"
  echo ""
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  echo ""
  echo "You can add the line above to your ~/.bashrc, ~/.zshrc, or profile file."
fi
