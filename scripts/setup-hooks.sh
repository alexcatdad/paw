#!/bin/bash
#
# Setup Git Hooks
# Installs pre-commit hook to run security checks before each commit
#

set -e

HOOK_DIR=".git/hooks"
HOOK_FILE="$HOOK_DIR/pre-commit"

echo "Setting up git hooks..."

# Create hooks directory if it doesn't exist
mkdir -p "$HOOK_DIR"

# Create pre-commit hook
cat > "$HOOK_FILE" << 'EOF'
#!/bin/bash
#
# Pre-commit hook: Run security checks
#

echo "Running pre-commit security checks..."

# Run security check script
if [ -f "./scripts/security-check.sh" ]; then
    ./scripts/security-check.sh
    if [ $? -ne 0 ]; then
        echo ""
        echo "Commit aborted due to security issues."
        echo "Fix the issues above or use --no-verify to skip (not recommended)."
        exit 1
    fi
fi

exit 0
EOF

chmod +x "$HOOK_FILE"

echo "Pre-commit hook installed at $HOOK_FILE"
echo ""
echo "The hook will run ./scripts/security-check.sh before each commit."
echo "To skip the hook (not recommended): git commit --no-verify"
