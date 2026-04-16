#!/usr/bin/env bash
set -e

# Configuration
REPO="enolalab/alfred"
BINARY_NAME="alfred"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 1. Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS_LOWER=linux;;
    Darwin*)    OS_LOWER=darwin;;
    *)          log_error "Unsupported OS: ${OS}"; exit 1;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)     ARCH_LOWER=amd64;;
    arm64|aarch64) ARCH_LOWER=arm64;;
    *)          log_error "Unsupported architecture: ${ARCH}"; exit 1;;
esac

log_info "Detected OS: ${OS_LOWER}, Architecture: ${ARCH_LOWER}"

# 3. Find latest release
log_info "Fetching latest release information..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "${LATEST_TAG}" ]; then
    log_error "Failed to fetch latest release tag. Check your internet connection or GitHub API limits."
    exit 1
fi

log_info "Latest release: ${LATEST_TAG}"

# Construct download URL 
# Note: Assuming standard GoReleaser asset naming convention.
if [ "${OS_LOWER}" = "darwin" ]; then
    OS_CAP="Darwin"
else
    OS_CAP="Linux"
fi

if [ "${ARCH_LOWER}" = "amd64" ]; then
    ARCH_CAP="x86_64"
else
    ARCH_CAP="arm64"
fi

TARBALL="${BINARY_NAME}_${OS_CAP}_${ARCH_CAP}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${TARBALL}"

# 4. Download and Extract
TMP_DIR=$(mktemp -d)
trap 'rm -rf -- "$TMP_DIR"' EXIT

log_info "Downloading ${DOWNLOAD_URL}..."
if ! curl -sL --fail "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}"; then
    log_error "Download failed. Please check if the release asset exists for your platform at:"
    log_error "${DOWNLOAD_URL}"
    exit 1
fi

log_info "Extracting archive..."
tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"

if [ ! -f "${TMP_DIR}/${BINARY_NAME}" ]; then
    log_error "Binary '${BINARY_NAME}' not found in the archive."
    exit 1
fi

# 5. Install
INSTALL_DIR="/usr/local/bin"
if [ ! -w "${INSTALL_DIR}" ]; then
    log_info "Requires root privileges to install to ${INSTALL_DIR}. Prompting for sudo..."
    SUDO="sudo"
else
    SUDO=""
fi

log_info "Installing ${BINARY_NAME} to ${INSTALL_DIR}..."
${SUDO} mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
${SUDO} chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# 6. Verify
if command -v ${BINARY_NAME} >/dev/null 2>&1; then
    log_success "Alfred installed successfully! (${LATEST_TAG})"
    log_info "To get started, run the interactive wizard:"
    echo "    alfred chat"
else
    log_error "Installation seemed to complete, but '${BINARY_NAME}' is not in your PATH."
    log_info "You may need to add ${INSTALL_DIR} to your PATH."
    exit 1
fi
