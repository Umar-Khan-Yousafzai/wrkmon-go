#!/bin/bash
# wrkmon-go uninstaller for Linux/macOS
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[*]${NC} $1"; }
ok()    { echo -e "${GREEN}[+]${NC} $1"; }

echo ""
info "Uninstalling wrkmon-go..."

# Remove binary
for dir in /usr/local/bin "$HOME/.local/bin"; do
    if [ -f "${dir}/wrkmon-go" ]; then
        if [ -w "${dir}" ]; then
            rm "${dir}/wrkmon-go"
        else
            sudo rm "${dir}/wrkmon-go"
        fi
        ok "Removed ${dir}/wrkmon-go"
    fi
done

# Remove config and data (ask first)
echo ""
info "Remove config and data? (~/.config/wrkmon-go and ~/.local/share/wrkmon-go)"
read -p "  [y/N] " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf "${HOME}/.config/wrkmon-go" "${HOME}/.local/share/wrkmon-go"
    ok "Removed config and data"
else
    info "Kept config and data"
fi

echo ""
ok "wrkmon-go uninstalled."
echo ""
