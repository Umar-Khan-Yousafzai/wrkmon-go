#!/bin/bash
# wrkmon-go installer for Linux/macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/install.sh | bash
set -euo pipefail

REPO="Umar-Khan-Yousafzai/wrkmon-go"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="wrkmon-go"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[*]${NC} $1"; }
ok()    { echo -e "${GREEN}[+]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
fail()  { echo -e "${RED}[x]${NC} $1"; exit 1; }

echo ""
echo -e "${CYAN}"
echo "              _                          "
echo " __      __ _ | | __ _ __   ___  _ __    "
echo " \ \ /\ / /| '__|| |/ /| '_ \ / _ \| '_ \   "
echo "  \ V  V / | |   |   < | | | | (_) | | | |  "
echo "   \_/\_/  |_|   |_|\_\|_| |_|\___/|_| |_|  "
echo -e "${NC}"
echo "  YouTube TUI Player — Installer"
echo ""

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)  GOOS="linux" ;;
    darwin) GOOS="darwin" ;;
    *)      fail "Unsupported OS: $OS. Use Windows installer for Windows." ;;
esac

case "$ARCH" in
    x86_64|amd64)   GOARCH="amd64" ;;
    aarch64|arm64)   GOARCH="arm64" ;;
    *)               fail "Unsupported architecture: $ARCH" ;;
esac

info "Detected: ${GOOS}/${GOARCH}"

# Get latest release tag
info "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "${LATEST:-}" ]; then
    fail "Could not determine latest release. Check https://github.com/${REPO}/releases"
fi
ok "Latest version: ${LATEST}"

# Download binary
ASSET="wrkmon-go-${GOOS}-${GOARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}"

info "Downloading ${ASSET}..."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if ! curl -fsSL -o "${TMP}/${BINARY}" "${DOWNLOAD_URL}"; then
    fail "Download failed. Check https://github.com/${REPO}/releases for available binaries."
fi
chmod +x "${TMP}/${BINARY}"
ok "Downloaded successfully"

# Install binary
info "Installing to ${INSTALL_DIR}/${BINARY}..."
if [ -w "${INSTALL_DIR}" ]; then
    mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
    sudo mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi
ok "Installed: ${INSTALL_DIR}/${BINARY}"

# Check dependencies
echo ""
info "Checking dependencies..."

check_dep() {
    if command -v "$1" &>/dev/null; then
        ok "$1 found: $(command -v "$1")"
        return 0
    else
        warn "$1 NOT found"
        return 1
    fi
}

MISSING=0

if ! check_dep mpv; then
    MISSING=1
    echo ""
    case "$GOOS" in
        linux)
            if command -v apt &>/dev/null; then
                warn "Install mpv:  sudo apt install mpv"
            elif command -v dnf &>/dev/null; then
                warn "Install mpv:  sudo dnf install mpv"
            elif command -v pacman &>/dev/null; then
                warn "Install mpv:  sudo pacman -S mpv"
            else
                warn "Install mpv from your package manager"
            fi
            ;;
        darwin)
            warn "Install mpv:  brew install mpv"
            ;;
    esac
fi

if ! check_dep yt-dlp; then
    MISSING=1
    echo ""
    case "$GOOS" in
        linux)
            if command -v pip3 &>/dev/null; then
                warn "Install yt-dlp:  pip3 install yt-dlp"
            elif command -v apt &>/dev/null; then
                warn "Install yt-dlp:  sudo apt install yt-dlp  (or pip3 install yt-dlp)"
            else
                warn "Install yt-dlp:  pip3 install yt-dlp"
            fi
            ;;
        darwin)
            warn "Install yt-dlp:  brew install yt-dlp"
            ;;
    esac
fi

echo ""
if [ "$MISSING" -eq 0 ]; then
    ok "All dependencies satisfied!"
    echo ""
    echo -e "  Run ${GREEN}wrkmon-go${NC} to start."
else
    warn "Install missing dependencies above, then run: wrkmon-go"
fi
echo ""
