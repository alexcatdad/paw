#!/bin/bash
#
# Security Check Script
# Scans for personal information that shouldn't be committed
#
# Usage: ./scripts/security-check.sh [--fix]
#

set -e

RED='\033[0;31m'
YELLOW='\033[0;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

FOUND_ISSUES=0

echo "Running security checks..."
echo ""

# Files to scan (exclude common non-code files)
FILES=$(git ls-files | grep -v -E '\.(png|jpg|jpeg|gif|ico|woff|woff2|ttf|eot|lock|lockb)$' | grep -v -E '^(node_modules/|dist/|\.git/)' || true)

if [ -z "$FILES" ]; then
    echo -e "${YELLOW}No files to scan${NC}"
    exit 0
fi

# ─────────────────────────────────────────────────────────────────────────────
# Check for hardcoded home directory paths
# ─────────────────────────────────────────────────────────────────────────────
echo "Checking for hardcoded home paths..."

# Pattern for personal home directories (not system paths like /home/linuxbrew)
# We look for /Users/<name>/ or /home/<name>/ but exclude known system paths
for file in $FILES; do
    if [[ "$file" =~ (security-check\.sh|\.md)$ ]]; then
        continue
    fi

    # Check for macOS personal paths
    MACOS_MATCHES=$(grep -n -E '/Users/[a-zA-Z][a-zA-Z0-9_-]+/' "$file" 2>/dev/null || true)
    if [ -n "$MACOS_MATCHES" ]; then
        echo -e "${RED}Found hardcoded macOS home paths in $file:${NC}"
        echo "$MACOS_MATCHES" | head -5 | sed 's/^/    /'
        FOUND_ISSUES=1
    fi

    # Check for Linux personal paths (excluding linuxbrew which is a system path)
    LINUX_MATCHES=$(grep -n -E '/home/[a-zA-Z][a-zA-Z0-9_-]+/' "$file" 2>/dev/null | \
        grep -v -E '/home/linuxbrew/' || true)
    if [ -n "$LINUX_MATCHES" ]; then
        echo -e "${RED}Found hardcoded Linux home paths in $file:${NC}"
        echo "$LINUX_MATCHES" | head -5 | sed 's/^/    /'
        FOUND_ISSUES=1
    fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Check for email addresses (excluding obvious fake/example ones)
# ─────────────────────────────────────────────────────────────────────────────
echo "Checking for email addresses..."
EMAIL_MATCHES=$(echo "$FILES" | xargs grep -l -E '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}' 2>/dev/null | grep -v -E '(security-check\.sh|\.md|package\.json)$' || true)

if [ -n "$EMAIL_MATCHES" ]; then
    for file in $EMAIL_MATCHES; do
        # Filter out common false positives:
        # - example.com, example.org domains
        # - noreply@, user@, test@, etc.
        # - git@github.com (SSH URLs, not emails)
        # - anthropic.com (Co-Authored-By)
        REAL_EMAILS=$(grep -E '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}' "$file" | \
            grep -v -E '(example\.com|example\.org|noreply@|user@|your\.email@|email@|test@|foo@|bar@|git@github\.com|git@gitlab\.com|git@bitbucket\.org|anthropic\.com)' || true)
        if [ -n "$REAL_EMAILS" ]; then
            echo -e "${YELLOW}Potential email addresses in $file:${NC}"
            echo "$REAL_EMAILS" | head -5 | sed 's/^/    /'
            FOUND_ISSUES=1
        fi
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
# Check for IP addresses (excluding localhost and common examples)
# ─────────────────────────────────────────────────────────────────────────────
echo "Checking for IP addresses..."
IP_MATCHES=$(echo "$FILES" | xargs grep -l -E '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' 2>/dev/null | grep -v -E '(security-check\.sh|\.md)$' || true)

if [ -n "$IP_MATCHES" ]; then
    for file in $IP_MATCHES; do
        # Filter out localhost, example IPs, and version numbers
        REAL_IPS=$(grep -E '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' "$file" | \
            grep -v -E '(127\.0\.0\.1|0\.0\.0\.0|192\.168\.|10\.0\.|172\.(1[6-9]|2[0-9]|3[0-1])\.|255\.255\.|1\.2\.3\.4|8\.8\.8\.8)' | \
            grep -v -E 'version|Version|VERSION' || true)
        if [ -n "$REAL_IPS" ]; then
            echo -e "${YELLOW}Potential IP addresses in $file:${NC}"
            echo "$REAL_IPS" | head -5 | sed 's/^/    /'
            FOUND_ISSUES=1
        fi
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
# Check for private keys
# ─────────────────────────────────────────────────────────────────────────────
echo "Checking for private keys..."
KEY_PATTERNS=(
    'BEGIN RSA PRIVATE KEY'
    'BEGIN OPENSSH PRIVATE KEY'
    'BEGIN EC PRIVATE KEY'
    'BEGIN DSA PRIVATE KEY'
    'BEGIN PGP PRIVATE KEY'
)

for pattern in "${KEY_PATTERNS[@]}"; do
    MATCHES=$(echo "$FILES" | xargs grep -l "$pattern" 2>/dev/null || true)
    if [ -n "$MATCHES" ]; then
        echo -e "${RED}Found private keys:${NC}"
        echo "$MATCHES" | sed 's/^/  /'
        FOUND_ISSUES=1
    fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Check for AWS credentials
# ─────────────────────────────────────────────────────────────────────────────
echo "Checking for AWS credentials..."
AWS_PATTERNS=(
    'AKIA[0-9A-Z]{16}'
    'aws_access_key_id\s*=\s*[A-Z0-9]+'
    'aws_secret_access_key\s*=\s*[A-Za-z0-9/+=]+'
)

for pattern in "${AWS_PATTERNS[@]}"; do
    MATCHES=$(echo "$FILES" | xargs grep -l -E "$pattern" 2>/dev/null | grep -v -E '(security-check\.sh|\.md)$' || true)
    if [ -n "$MATCHES" ]; then
        echo -e "${RED}Found potential AWS credentials:${NC}"
        echo "$MATCHES" | sed 's/^/  /'
        FOUND_ISSUES=1
    fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Check for API keys / tokens
# ─────────────────────────────────────────────────────────────────────────────
echo "Checking for API keys..."
API_PATTERNS=(
    'api[_-]?key\s*[:=]\s*["\x27][a-zA-Z0-9_-]{20,}["\x27]'
    'api[_-]?token\s*[:=]\s*["\x27][a-zA-Z0-9_-]{20,}["\x27]'
    'bearer\s+[a-zA-Z0-9_-]{20,}'
)

for pattern in "${API_PATTERNS[@]}"; do
    MATCHES=$(echo "$FILES" | xargs grep -l -iE "$pattern" 2>/dev/null | grep -v -E '(security-check\.sh|\.md)$' || true)
    if [ -n "$MATCHES" ]; then
        echo -e "${YELLOW}Potential API keys in:${NC}"
        echo "$MATCHES" | sed 's/^/  /'
        FOUND_ISSUES=1
    fi
done

# ─────────────────────────────────────────────────────────────────────────────
# Results
# ─────────────────────────────────────────────────────────────────────────────
echo ""
if [ $FOUND_ISSUES -eq 0 ]; then
    echo -e "${GREEN}No security issues found.${NC}"
    exit 0
else
    echo -e "${RED}Security issues found! Please review and fix before committing.${NC}"
    exit 1
fi
