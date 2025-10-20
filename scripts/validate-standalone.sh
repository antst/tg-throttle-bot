#!/bin/bash
# validate-standalone.sh
# Validates that standalone documentation files contain no internal development references
#
# Usage: ./scripts/validate-standalone.sh
# Exit codes:
#   0 - All checks passed (no violations found)
#   1 - Violations found (prints list of matches)

set -euo pipefail

# Change to repository root
cd "$(dirname "$0")/.."

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Files to check
STANDALONE_FILES=(
    "CHANGELOG-STANDALONE.md"
    "DEPLOYMENT-STANDALONE.md"
    "README-STANDALONE.md"
)

# Forbidden patterns (case-insensitive for most, case-sensitive for Feature numbers)
FORBIDDEN_PATTERNS=(
    "spec-kit"
    "specs/"
    "docs/"
)

# Feature number pattern (case-sensitive)
FEATURE_PATTERN='Feature 0[0-9][0-9]'

# Track violations
VIOLATIONS_FOUND=0
VIOLATION_DETAILS=()

echo "=================================================="
echo "Validating Standalone Documentation Files"
echo "=================================================="
echo ""

# Check if standalone files exist
MISSING_FILES=0
for file in "${STANDALONE_FILES[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo -e "${YELLOW}⚠ Warning:${NC} File not found: $file (skipping)"
        MISSING_FILES=$((MISSING_FILES + 1))
    fi
done

if [[ $MISSING_FILES -eq ${#STANDALONE_FILES[@]} ]]; then
    echo -e "${RED}✗ FAIL:${NC} No standalone files found"
    exit 1
fi

echo "Checking for forbidden terms..."
echo ""

# Check each pattern
for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
    echo "Searching for: '$pattern' (case-insensitive)"
    
    for file in "${STANDALONE_FILES[@]}"; do
        if [[ ! -f "$file" ]]; then
            continue
        fi
        
        # Search case-insensitively
        matches=$(grep -i -n "$pattern" "$file" 2>/dev/null || true)
        
        if [[ -n "$matches" ]]; then
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
            echo -e "  ${RED}✗${NC} Found in $file:"
            while IFS= read -r line; do
                VIOLATION_DETAILS+=("$file: $line")
                echo "    Line $line"
            done <<< "$matches"
        fi
    done
done

# Check for Feature numbers (case-sensitive)
echo ""
echo "Searching for: '$FEATURE_PATTERN' (case-sensitive)"

for file in "${STANDALONE_FILES[@]}"; do
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    # Search case-sensitively for "Feature 0XX" pattern
    matches=$(grep -n "$FEATURE_PATTERN" "$file" 2>/dev/null || true)
    
    if [[ -n "$matches" ]]; then
        VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        echo -e "  ${RED}✗${NC} Found in $file:"
        while IFS= read -r line; do
            VIOLATION_DETAILS+=("$file: $line")
            echo "    Line $line"
        done <<< "$matches"
    fi
done

# Print summary
echo ""
echo "=================================================="
echo "Validation Summary"
echo "=================================================="

if [[ $VIOLATIONS_FOUND -eq 0 ]]; then
    echo -e "${GREEN}✓ PASS:${NC} No forbidden terms found in standalone files"
    echo ""
    echo "Checked patterns:"
    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        echo "  - '$pattern'"
    done
    echo "  - '$FEATURE_PATTERN'"
    echo ""
    echo "Files checked:"
    for file in "${STANDALONE_FILES[@]}"; do
        if [[ -f "$file" ]]; then
            echo "  - $file"
        fi
    done
    exit 0
else
    echo -e "${RED}✗ FAIL:${NC} Found $VIOLATIONS_FOUND violation(s)"
    echo ""
    echo "Violations:"
    for detail in "${VIOLATION_DETAILS[@]}"; do
        echo "  - $detail"
    done
    echo ""
    echo "Action required: Remove or replace forbidden terms in standalone files"
    exit 1
fi
