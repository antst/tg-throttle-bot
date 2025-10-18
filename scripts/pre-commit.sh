#!/bin/bash
# Pre-commit hook for throttleBot
# Runs formatting checks, linting, and unit tests before allowing commit

set -e

echo "🔍 Running pre-commit checks..."
echo ""

# Change to repository root
cd "$(git rev-parse --show-toplevel)"

# Run gofmt check
echo "1/4 Checking Go formatting..."
UNFORMATTED=$(gofmt -l . 2>&1 | grep -v "^vendor/" | grep -v "^internal/storage/sqlc/" || true)
if [ -n "$UNFORMATTED" ]; then
    echo "❌ The following files need formatting:"
    echo "$UNFORMATTED"
    echo ""
    echo "Run 'make fmt' or 'gofmt -w .' to fix"
    exit 1
fi
echo "✓ Formatting check passed"
echo ""

# Run go mod tidy check
echo "2/4 Checking go.mod..."
go mod tidy
if ! git diff --exit-code go.mod go.sum > /dev/null 2>&1; then
    echo "❌ go.mod or go.sum needs updating"
    echo "Changes have been applied. Please add them to your commit:"
    echo "  git add go.mod go.sum"
    exit 1
fi
echo "✓ go.mod check passed"
echo ""

# Run golangci-lint if available
echo "3/4 Running linter..."
if command -v golangci-lint &> /dev/null; then
    # Only lint main code and unit tests, skip integration/contract tests
    golangci-lint run --timeout=5m ./cmd/... ./internal/... ./tests/unit/...
    echo "✓ Linting passed"
else
    echo "⚠️  golangci-lint not found, skipping lint check"
    echo "   Install: brew install golangci-lint"
fi
echo ""

# Run unit tests
echo "4/4 Running unit tests..."
# if ! go test -race -short ./tests/unit/... 2>&1; then
if ! go test  -short ./tests/unit/... 2>&1; then
    echo "❌ Unit tests failed"
    echo ""
    echo "Fix the failing tests before committing"
    exit 1
fi
echo "✓ Unit tests passed"
echo ""

echo "✅ All pre-commit checks passed!"
exit 0
