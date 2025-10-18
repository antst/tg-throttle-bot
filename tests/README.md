# Testing Guide

## Testing Approach

This project uses **MCP (Model Context Protocol) testing** as the primary testing strategy for multi-window rate limiting functionality. Traditional unit and integration tests were replaced by comprehensive MCP test scenarios that validate the entire system end-to-end.

### MCP Testing (Primary)

**Test Report**: See [docs/archive/test-reports/MCP_TEST_REPORT.md](../docs/archive/test-reports/MCP_TEST_REPORT.md)

**Coverage**: 26+ test cases across all user stories:
- User Story 1+2 (Configuration + Enforcement): 8/8 tests ✅
- User Story 3 (Status Tracking): All tests passed ✅
- User Story 4 (Warning System): 7/10 tests passed (3 minor non-critical issues)
- 3-State Override System: All tests passed ✅
- Username Harvesting: Infrastructure complete ✅

**How to run**:
```bash
# Start the bot with Docker Compose
docker compose up -d

# Run MCP test script
./scripts/test-phase4-mcp.sh
```

### Unit Tests (`tests/unit/`)

**Purpose**: Test isolated logic components (error handling, duration parsing, config validation)

**Characteristics**:
- **No external dependencies** (no database, no Telegram API)
- Fast execution (< 1 second)
- Focus on pure functions and logic

**Run unit tests:**
```bash
go test -v ./tests/unit/...
```

**Active Tests**:
- `errors_test.go` - Error sanitization and user-friendly messages
- `duration_test.go` - Duration parsing/calculation/formatting (REGRESSION-001 fix)
- `config_*_test.go` - Configuration loading and validation
- `mock_client_test.go` - Mock Telegram client behavior

**Removed Tests** (2025-10-18):
- ~~`calculator_multiwindow_test.go`~~ - All tests were skipped placeholders
- ~~`calculator_test.go`~~ - Mock implementations not testing real code
- ~~`validator_test.go`~~ - All tests were skipped placeholders
- ~~`stats_test.go`~~ - Mock implementations not testing real code
- ~~`status_test.go`~~ - Mock implementations not testing real code

### Integration Tests (`tests/integration/`)

**Status**: ⚠️ **REMOVED** (2025-10-18)

All integration tests were skipped placeholders (27 tests total). MCP testing provides equivalent or better coverage of integration scenarios with real database and Telegram API interactions.

### Contract Tests (`tests/contract/`)

**Status**: ⚠️ **REMOVED** (2025-10-18)

All contract tests were skipped placeholders (48 tests total). Contract requirements are validated through MCP test scenarios that exercise the actual bot commands and API interactions.

## Test Configuration

### Config Factory for Unit Tests

The `config.NewTestConfig()` function creates minimal test configurations:

```go
// Basic test config (no external dependencies)
cfg := config.NewTestConfig(
    config.WithDatabaseURL("postgresql://localhost/testdb"),
)

// Verify config values
assert.NotEmpty(t, cfg.DatabaseURL)
assert.NotEmpty(t, cfg.CharLimit) // Has sensible defaults
```

### Mock Telegram Client

The mock client simulates Telegram API behavior for testing:

```go
// Create mock client
mockClient := telegram.NewMockClient()

// Configure expected behavior
mockClient.SetAdminStatus(chatID, userID, true)

// Use in code under test
isAdmin, err := mockClient.IsAdmin(ctx, chatID, userID)

// Verify interactions
assert.Equal(t, 1, len(mockClient.IsAdminCalls))
```

## CI/CD Integration

### Pre-commit Hook

Runs linting and unit tests:
```bash
./scripts/pre-commit.sh
```

### CI Pipeline

**Linting** (always run):
```yaml
- name: Lint code
  run: make lint
```

**Unit tests** (always run):
```yaml
- name: Run unit tests
  run: go test -v ./tests/unit/...
```

**Build verification**:
```yaml
- name: Build bot
  run: go build -o bin/bot ./cmd/bot
```

## MCP Testing Setup

### Prerequisites

1. **Docker Compose** (for PostgreSQL and bot)
2. **Telegram Bot Token** (configured in `.env` or `docker-compose.yml`)
3. **Test Group** (with bot as admin)

### Running MCP Tests

```bash
# 1. Start services
docker compose up -d

# 2. Verify bot is running
docker compose logs bot --tail 20

# 3. Run MCP test script
./scripts/test-phase4-mcp.sh

# 4. Monitor results
# Tests interact with actual Telegram group
# Validates: commands, enforcement, warnings, status tracking
```

### MCP Test Coverage

**Configuration Tests**:
- `/setwindow` with arbitrary durations (30m, 2h, 7d)
- `/enablewindow` and `/disablewindow` commands
- Window configuration persistence

**Enforcement Tests**:
- Message deletion when ANY window exceeds limit
- Auto-recovery when messages age out
- Multi-window evaluation (A→B→C sequential)

**Status Tests**:
- `/mystatus` per-window breakdown
- `/windows` configuration display
- Real-time usage calculation

**Warning Tests**:
- 80%, 90%, 100% threshold warnings
- Per-window warning independence
- Ephemeral message auto-delete (7 seconds)

**Override Tests**:
- 3-state system (whitelist/blacklist/normal)
- Temporary overrides with expiration
- Automatic cleanup (every 5 minutes)

## Coverage

### Unit Test Coverage

```bash
# Run unit tests with coverage
go test -coverprofile=coverage.out ./tests/unit/...

# View coverage report
go tool cover -html=coverage.out
```

### MCP Test Coverage

**Metrics** (from MCP_TEST_REPORT.md):
- Total test cases: 26+
- Passed: 23 ✅
- Minor issues: 3 (non-critical formatting)
- Pass rate: ~88%

**Functional Requirements Coverage**:
- FR-001 to FR-024: All validated ✅
- Edge cases: 10+ scenarios tested
- Performance: <500ms window evaluation ✅

## Troubleshooting

### Unit Tests

**"go test ./tests/unit/... fails"**
- Check Go version (requires Go 1.21+)
- Verify dependencies: `go mod tidy`
- Run with verbose output: `go test -v ./tests/unit/...`

**"cannot import internal package"**
- Verify you're in the project root directory
- Check go.mod module name matches imports
- Run: `go mod verify`

### MCP Tests

**"Bot not responding in Telegram"**
- Check bot is running: `docker compose ps`
- Check logs: `docker compose logs bot --tail 50`
- Verify TELEGRAM_TOKEN is set correctly
- Ensure bot is admin in test group

**"Database migration errors"**
- Check migration version: `docker compose exec postgres psql -U throttle -d throttlebot -c "SELECT version, dirty FROM schema_migrations"`
- Expected: `version=7, dirty=false`
- If dirty: Stop bot, fix migration, restart

**"Commands return errors"**
- Check database connectivity
- Verify group configuration in database
- Check bot permissions (must be admin)
- Review error logs for specific issues

**"Tests inconsistent results"**
- Clear database state between test runs
- Use unique test groups for isolation
- Check for rate limiting by Telegram API
- Verify clock synchronization (for time-based tests)

## Examples

### Unit Test Example

```go
// Test duration parsing (REGRESSION-001 fix)
func TestParseDurationString(t *testing.T) {
    value, unit, err := ratelimit.ParseDurationString("30m")
    assert.NoError(t, err)
    assert.Equal(t, 30, value)
    assert.Equal(t, "minute", unit)
}
```

### Mock Client Example

```go
// Test admin permission check
func TestAdminCommands(t *testing.T) {
    mockClient := telegram.NewMockClient()
    mockClient.SetAdminStatus(chatID, adminID, true)
    
    isAdmin, err := mockClient.IsAdmin(ctx, chatID, adminID)
    
    assert.NoError(t, err)
    assert.True(t, isAdmin)
    assert.Equal(t, 1, len(mockClient.IsAdminCalls))
}
```

### Error Sanitization Example

```go
// Test user-friendly error messages
func TestSanitizeError(t *testing.T) {
    err := fmt.Errorf("postgres://user:pass@localhost")
    result := bot.SanitizeError(err)
    
    // Should not leak connection string
    assert.NotContains(t, result, "postgres://")
    assert.NotContains(t, result, "pass")
    
    // Should return user-friendly message
    assert.Contains(t, result, "❌")
}
```

## Test Philosophy

### Why MCP Testing?

1. **End-to-end validation**: Tests the complete user journey, not isolated units
2. **Real environment**: Uses actual Telegram API and PostgreSQL database
3. **User perspective**: Validates what users actually experience
4. **Comprehensive**: Single test validates multiple requirements simultaneously
5. **Maintenance**: Fewer tests to maintain, closer to production behavior

### When to Use Unit Tests

Unit tests are appropriate for:
- **Pure functions**: Duration parsing, calculations, formatting
- **Error handling**: Sanitization, user-friendly messages
- **Configuration**: Validation, defaults, environment variables
- **Logic isolation**: Business rules that don't require external dependencies

### When to Use MCP Tests

MCP tests are appropriate for:
- **Feature validation**: Complete user stories (configuration, enforcement, status)
- **Integration scenarios**: Multi-window evaluation, auto-recovery, warnings
- **Performance**: Real-world response times under actual database load
- **Regression prevention**: High-level behavior preserved across refactoring

