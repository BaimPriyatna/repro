#!/bin/sh
set -eu

REPO="BaimPriyatna/repro"

# 1. Detect Operating System
OS_RAW="$(uname -s)"
case "${OS_RAW}" in
  Linux)
    OS="linux"
    ;;
  Darwin)
    OS="darwin"
    ;;
  *)
    echo "Error: Unsupported operating system: ${OS_RAW}" >&2
    echo "Supported operating systems: Linux, Darwin" >&2
    exit 1
    ;;
esac

# 2. Detect Architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "Error: Unsupported architecture: ${ARCH_RAW}" >&2
    echo "Supported architectures: x86_64 (amd64), aarch64 (arm64)" >&2
    exit 1
    ;;
esac

# 3. Determine Version
if [ -n "${REPRO_VERSION:-}" ]; then
  VERSION="${REPRO_VERSION}"
else
  # Resolve latest version tag from GitHub
  echo "Finding latest release of ${REPO}..."
  VERSION="$(curl -fsSL -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest" 2>/dev/null | sed 's#.*/tag/##')"
  if [ -z "${VERSION}" ] || [ "${VERSION}" = "https://github.com/${REPO}/releases/latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  fi
  if [ -z "${VERSION}" ]; then
    echo "Error: Unable to determine latest release version from GitHub." >&2
    echo "Please specify REPRO_VERSION manually (e.g. REPRO_VERSION=v1.1.0)." >&2
    exit 1
  fi
fi

# Ensure version has 'v' prefix if omitted
case "${VERSION}" in
  v*) ;;
  *) VERSION="v${VERSION}" ;;
esac

ARCHIVE_NAME="repro_${OS}_${ARCH}.tar.gz"
CHECKSUMS_NAME="checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
DOWNLOAD_URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUMS_URL="${BASE_URL}/${CHECKSUMS_NAME}"

# 4. Setup Temporary Directory with Cleanup
TMP_DIR="$(mktemp -d -t repro-install.XXXXXX 2>/dev/null || mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM HUP

# 5. Download Archive and Checksums (fail-fast)
echo "Downloading Repro ${VERSION} (${OS}/${ARCH})..."
if ! curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
  echo "Error: Failed to download ${DOWNLOAD_URL}" >&2
  exit 1
fi

echo "Downloading ${CHECKSUMS_NAME}..."
if ! curl -fsSL "${CHECKSUMS_URL}" -o "${TMP_DIR}/${CHECKSUMS_NAME}"; then
  echo "Error: Failed to download ${CHECKSUMS_URL}" >&2
  exit 1
fi

# 6. Verify Checksum
echo "Verifying SHA-256 checksum..."
EXPECTED_HASH="$(grep "  ${ARCHIVE_NAME}\$" "${TMP_DIR}/${CHECKSUMS_NAME}" | awk '{print $1}')"
if [ -z "${EXPECTED_HASH}" ]; then
  EXPECTED_HASH="$(grep "${ARCHIVE_NAME}" "${TMP_DIR}/${CHECKSUMS_NAME}" | awk '{print $1}')"
fi

if [ -z "${EXPECTED_HASH}" ]; then
  echo "Error: Checksum entry for ${ARCHIVE_NAME} not found in ${CHECKSUMS_NAME}." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_HASH="$(sha256sum "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_HASH="$(shasum -a 256 "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')"
else
  echo "Error: Neither sha256sum nor shasum is available to verify the checksum." >&2
  exit 1
fi

if [ "${EXPECTED_HASH}" != "${ACTUAL_HASH}" ]; then
  echo "Error: SHA-256 checksum verification failed!" >&2
  echo "  Expected: ${EXPECTED_HASH}" >&2
  echo "  Got:      ${ACTUAL_HASH}" >&2
  exit 1
fi
echo "Checksum verification successful."

# 7. Extract Archive
echo "Extracting binary..."
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "${TMP_DIR}"
if [ ! -f "${TMP_DIR}/repro" ]; then
  echo "Error: Executable binary 'repro' not found in downloaded archive." >&2
  exit 1
fi
chmod +x "${TMP_DIR}/repro"

# 8. Install Binary
TARGET_PREFERRED="/usr/local/bin"
TARGET_FALLBACK="${HOME}/.local/bin"
INSTALL_DIR=""

if [ -w "${TARGET_PREFERRED}" ] || { [ ! -d "${TARGET_PREFERRED}" ] && mkdir -p "${TARGET_PREFERRED}" 2>/dev/null && [ -w "${TARGET_PREFERRED}" ]; }; then
  INSTALL_DIR="${TARGET_PREFERRED}"
else
  mkdir -p "${TARGET_FALLBACK}"
  INSTALL_DIR="${TARGET_FALLBACK}"
fi

echo "Installing repro to ${INSTALL_DIR}..."
cp "${TMP_DIR}/repro" "${INSTALL_DIR}/repro"
chmod +x "${INSTALL_DIR}/repro"
echo "Repro ${VERSION} installed successfully to ${INSTALL_DIR}/repro"

# 9. Verify PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    ;;
  *)
    echo ""
    echo "Notice: ${INSTALL_DIR} is not currently in your PATH."
    echo "To use 'repro', add ${INSTALL_DIR} to PATH by running:"
    echo "  export PATH=\"\${PATH}:${INSTALL_DIR}\""
    echo "or add that line to your ~/.bashrc or ~/.zshrc."
    ;;
esac
