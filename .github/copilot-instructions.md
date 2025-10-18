# throttleBot Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-10-18

## Active Technologies
- Go 1.25 (002-message-rate-limiter)
- PostgreSQL with multi-window schema (006-multi-window-rate-limiting)
- Multi-window rate limiting with arbitrary durations (REGRESSION-001 resolved)
- Embedded migrations (go:embed) for single-binary deployment
- SQLC with directory-based schema configuration

## Project Structure
```
cmd/bot/          - Main application entry point
internal/
  bot/            - Telegram bot handlers and commands (parser, calculator, formatter)
  ratelimit/      - Multi-window rate limiting logic
  storage/        - PostgreSQL database operations (SQLC generated)
  telegram/       - Telegram API client
  metrics/        - Prometheus metrics
  logging/        - Structured logging
migrations/       - Database schema migrations (embedded with go:embed)
  embed.go        - Embedded FS for migrations
docs/             - Documentation
  EMBEDDED_MIGRATIONS.md  - Embedded migrations guide
  MCP_TEST_REPORT.md      - MCP testing results
  REGRESSION-001-RESOLUTION.md - Arbitrary duration resolution
tests/
  unit/           - Unit tests
  integration/    - Integration tests
  contract/       - Contract tests
```

## Architecture

### Rate Limiting (Multi-Window Only)
- **Current**: Multi-window rate limiting with 3 independent slots (A/B/C) per group
- **Durations**: Arbitrary durations from 1-365 units (minute/hour/day)
- **Schema**: `duration_value` + `duration_unit` → `window_duration` (computed seconds)
- **Removed**: Legacy single-window code (688 lines removed 2025-10-17)
- **Database**: `window_slots` table is the ONLY source of rate limit configuration
- **Flow**: `handleMessage()` → `handleMessageMultiWindow()` → evaluates ALL enabled windows

### Duration Support (REGRESSION-001 Resolved)
- **Parser**: `ParseDurationString("30m")` → (30, "minute")
- **Calculator**: `CalculateWindowDuration(30, "minute")` → 1800 seconds
- **Formatter**: `FormatWindowDuration(30, "minute")` → "30 minutes" (with pluralization)
- **Validation**: 1 ≤ value ≤ 365, unit ∈ {minute, hour, day}
- **Syntax**: `/setwindow a 1000 30m` (supports Xm, Xh, Xd format)

### Commands
**Multi-Window Commands** (Current):
- `/setwindow <a|b|c> <limit> <duration>` - Configure window slot
  * Examples: `/setwindow a 1000 30m`, `/setwindow b 5000 2h`, `/setwindow c 10000 7d`
  * Duration format: `<1-365><m|h|d>` (e.g., 30m, 2h, 7d)
- `/showwindows` - Display all window configurations
- `/enablewindow <a|b|c>` - Enable a window slot
- `/disablewindow <a|b|c>` - Disable a window slot

**Removed Commands** (Legacy):
- ~~`/setlimit`~~ - Removed 2025-10-17
- ~~`/showconfig`~~ - Removed 2025-10-17
- ~~`/mystatus`~~ - Removed 2025-10-17
- ~~`/resetconfig`~~ - Removed 2025-10-17

## Code Style
- Go 1.25: Follow standard conventions
- Use structured logging (zap)
- Include context in all database operations
- Write tests before implementation (TDD)
- SQLC for type-safe database queries
- Embedded resources with go:embed (migrations, assets)

## Build & Deployment
- **Migrations**: Embedded in binary using `//go:embed *.sql` in `migrations/embed.go`
- **SQLC**: Directory-based schema (`schema: "migrations"` in sqlc.yaml)
- **Docker**: Single-stage build, migrations embedded, 38.4MB image
- **Binary**: Single binary with all migrations included (no runtime SQL files needed)

## Recent Changes
- 2025-10-18: **✅ PRODUCTION READY** - Multi-window rate limiting feature complete
- 2025-10-18: **DOCUMENTATION CLEANUP** - Archived 32 obsolete files, created concise status docs
- 2025-10-18: **SCHEMA CLEANUP** - Removed exemptions table (migration 000007, cleaned up migrations)
- 2025-10-18: **AUTOMATIC CLEANUP VERIFIED** - Expired overrides removed every 5 minutes (tested & working)
- 2025-10-18: **SLOT_ID OPTIMIZATION** - Single message log shared across all windows (migration 000006, 66% storage reduction)
- 2025-10-18: **USER STORY 6** - @username syntax infrastructure with automatic harvesting
- 2025-10-18: **REGRESSION-001 RESOLVED** - Arbitrary duration support (1-365 minutes/hours/days)
- 2025-10-18: Implemented embedded migrations with go:embed (single binary deployment)
- 2025-10-18: MCP testing: 26+ test cases, 100% pass rate, zero known bugs
- 2025-10-17: **MAJOR CLEANUP** - Removed ~1,400 lines of legacy/dead code
- 2025-10-17: Completed User Stories 1-4 (Configuration, Enforcement, Status, Warnings)

## Database Schema (6 Tables)
- `users` - User records with username harvesting
- `groups` - Group configuration (pause/resume state)
- `window_slots` - Multi-window rate limit configuration (3 slots per group)
- `simple_messages` - Single message log (shared across all windows, optimized with migration 000006)
- `user_overrides` - Manual user exemptions (3-state: nil/whitelist/blacklist, with expiration)
- `schema_migrations` - Migration version tracking

**Removed Tables:**
- ~~`exemptions`~~ - Removed in migration 000007 (never used, 0 rows, replaced by user_overrides)

## Key Features (Production Ready)
- ✅ Multi-window rate limiting (3 independent windows per group)
- ✅ Arbitrary durations (1-365 minutes/hours/days)
- ✅ @username syntax in admin commands (automatic username harvesting)
- ✅ Single message log optimization (66% storage reduction)
- ✅ Automatic expired override cleanup (every 5 minutes)
- ✅ Embedded migrations (no runtime SQL files needed)
- ✅ Type-safe database operations (SQLC)
- ✅ Comprehensive validation and error handling
- ✅ Single binary deployment (38.4MB Docker image)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
