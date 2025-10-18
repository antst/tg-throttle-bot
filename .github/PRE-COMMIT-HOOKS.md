# Pre-commit Hooks for throttleBot

## Overview

Pre-commit hooks automatically run code quality checks before each commit to ensure consistent code quality and prevent broken code from being committed.

## What Gets Checked

The pre-commit hook performs 4 checks:

1. **Go Formatting** - Ensures all Go code is properly formatted with `gofmt`
2. **Go Modules** - Verifies `go.mod` and `go.sum` are up to date
3. **Linting** - Runs `golangci-lint` on main code and unit tests
4. **Unit Tests** - Runs all unit tests with race detector

## Installation

The pre-commit hook has already been installed automatically. To reinstall or update:

```bash
make install-hooks
```

Or manually:

```bash
cp scripts/pre-commit.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## Running Manually

To run the pre-commit checks without committing:

```bash
# Run all pre-commit checks
make pre-commit

# Or run the script directly
bash scripts/pre-commit.sh
```

## Running Individual Checks

```bash
# Format code
make fmt

# Run linter
make lint

# Run unit tests only
make test-unit

# Run all tests
make test-all
```

## Bypassing Pre-commit Hooks (Not Recommended)

In rare cases where you need to commit without running hooks:

```bash
git commit --no-verify -m "your message"
```

**Warning:** Only use `--no-verify` in exceptional circumstances. Fix the issues instead.

## Current Linting Issues

As of the last check, there are **49 linting issues** that should be addressed:

- **errcheck (6)**: Unchecked error returns (e.g., `w.Write()`, `rows.Close()`)
- **errorlint (5)**: Incorrect error comparison (use `errors.Is` instead of `==`)
- **goconst (5)**: Repeated strings that should be constants
- **gocritic (2)**: Code quality suggestions
- **gosec (4)**: Security concerns (integer overflow, missing ReadHeaderTimeout)
- **noctx (1)**: Missing context.Context usage
- **revive (24)**: Style issues (missing package comments, unused parameters, etc.)
- **staticcheck (1)**: Code simplification suggestions
- **unused (1)**: Unused field

## Fixing Linting Issues

To see detailed linting issues:

```bash
make lint
```

Common fixes:

### 1. Unchecked Errors
```go
// Bad
w.Write([]byte("OK"))

// Good
_, _ = w.Write([]byte("OK"))
// or
if _, err := w.Write([]byte("OK")); err != nil {
    // handle error
}
```

### 2. Error Comparison
```go
// Bad
if err == sql.ErrNoRows {

// Good
if errors.Is(err, sql.ErrNoRows) {
```

### 3. Repeated Strings
```go
// Bad
if windowType != "day" { ... }
if x == "day" { ... }

// Good
const WindowTypeDay = "day"
if windowType != WindowTypeDay { ... }
```

## Integration with CI/CD

The same checks run in CI/CD pipelines to ensure code quality. Pull requests must pass all checks before merging.

## Troubleshooting

### Hook doesn't run
- Verify the hook is executable: `ls -la .git/hooks/pre-commit`
- Reinstall: `make install-hooks`

### golangci-lint not found
```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

### Tests fail
Fix the failing tests before committing. To see detailed test output:
```bash
go test -v -race ./tests/unit/...
```

## Configuration Files

- `.golangci.yml` - Linter configuration
- `scripts/pre-commit.sh` - Pre-commit hook script
- `Makefile` - Convenient make targets
- `.pre-commit-config.yaml` - Optional pre-commit framework configuration

## Optional: Pre-commit Framework

For more advanced hooks, you can use the pre-commit framework:

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install

# Run on all files
pre-commit run --all-files
```

