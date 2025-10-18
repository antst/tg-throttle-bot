#!/bin/bash
# Script to capture and display Go build errors properly

set -e

# Change to project root
cd "$(dirname "$0")/.."

echo "=== Building project ==="
echo ""

# Capture build output to both stdout and a temp file
BUILD_OUTPUT=$(mktemp)
trap "rm -f $BUILD_OUTPUT" EXIT

# Try to build and capture all output
if go build -v ./cmd/bot 2>&1 | tee "$BUILD_OUTPUT"; then
    echo ""
    echo "=== BUILD SUCCESS ==="
    exit 0
else
    echo ""
    echo "=== BUILD FAILED ==="
    cat "$BUILD_OUTPUT"
    exit 1
fi

