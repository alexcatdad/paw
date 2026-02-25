#!/usr/bin/env bash
# ══════════════════════════════════════════════════════════════════════════════
# PAW - Dotfiles Manager Installation Script
# Run this to install the paw CLI via Homebrew tap:
#   ./install.sh
# ══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}${BOLD}🐱 paw${NC} - dotfiles manager"
echo ""

if ! command -v brew >/dev/null 2>&1; then
  echo -e "${RED}✗${NC} Homebrew is required but not installed."
  echo -e "${YELLOW}→${NC} Install Homebrew first: https://brew.sh"
  exit 1
fi

echo -e "${GREEN}→${NC} Tapping alexcatdad/tap..."
brew tap alexcatdad/tap >/dev/null

echo -e "${GREEN}→${NC} Installing paw with Homebrew..."
if brew list --formula alexcatdad/tap/paw >/dev/null 2>&1; then
  brew upgrade alexcatdad/tap/paw || true
else
  brew install alexcatdad/tap/paw
fi

if ! brew list --formula alexcatdad/tap/paw >/dev/null 2>&1; then
  echo -e "${RED}✗${NC} Homebrew formula alexcatdad/tap/paw is not installed."
  exit 1
fi

if ! command -v paw >/dev/null 2>&1; then
  echo -e "${RED}✗${NC} paw is not available in PATH after install."
  exit 1
fi

if ! paw --version >/dev/null 2>&1; then
  echo -e "${RED}✗${NC} paw installed but failed version check."
  exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Show next steps
# ─────────────────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}✓ Installation complete!${NC}"
echo -e "${GREEN}→${NC} Installed version: $(paw --version)"
echo ""
echo -e "${BOLD}Next steps:${NC}"
echo -e "  ${CYAN}paw init <dotfiles-repo-url>${NC}  # Set up your dotfiles"
echo -e "  ${CYAN}paw --help${NC}                    # See all commands"
echo ""
