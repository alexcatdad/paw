#!/usr/bin/env bash
# ══════════════════════════════════════════════════════════════════════════════
# PAW - Dotfiles Manager Installation Script
# Run this to install the paw CLI:
#   curl -fsSL https://raw.githubusercontent.com/alexcatdad/paw/main/install.sh | bash
# ══════════════════════════════════════════════════════════════════════════════

set -e

REPO="alexcatdad/paw"
BIN_DIR="$HOME/.local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}${BOLD}🐱 paw${NC} - dotfiles manager"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Detect platform and architecture
# ─────────────────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Normalize architecture names
case "$ARCH" in
  x86_64) ARCH="x64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

echo -e "${GREEN}→${NC} Detected: ${OS}-${ARCH}"

# ─────────────────────────────────────────────────────────────────────────────
# Create bin directory
# ─────────────────────────────────────────────────────────────────────────────
mkdir -p "$BIN_DIR"

# ─────────────────────────────────────────────────────────────────────────────
# Download pre-built binary
# ─────────────────────────────────────────────────────────────────────────────
BINARY="paw-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"

echo -e "${GREEN}→${NC} Downloading paw..."
if curl -fsSL "$DOWNLOAD_URL" -o "$BIN_DIR/paw" 2>/dev/null; then
  chmod +x "$BIN_DIR/paw"
  echo -e "${GREEN}✓${NC} Installed paw to $BIN_DIR/paw"
else
  echo -e "${RED}✗${NC} Failed to download paw binary for ${OS}-${ARCH}"
  echo -e "${YELLOW}→${NC} You may need to build from source:"
  echo -e "   git clone https://github.com/${REPO}.git"
  echo -e "   cd paw && go build -o paw ./cmd/paw && mv paw ~/.local/bin/paw"
  exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# Ensure ~/.local/bin is in PATH
# ─────────────────────────────────────────────────────────────────────────────
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  echo ""
  echo -e "${YELLOW}Note:${NC} $BIN_DIR is not in your PATH"
  echo -e "Add this to your shell config:"
  echo -e "  ${CYAN}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Show next steps
# ─────────────────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}✓ Installation complete!${NC}"
echo ""
echo -e "${BOLD}Next steps:${NC}"
echo -e "  ${CYAN}paw init <dotfiles-repo-url>${NC}  # Set up your dotfiles"
echo -e "  ${CYAN}paw --help${NC}                    # See all commands"
echo ""
