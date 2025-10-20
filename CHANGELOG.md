# Changelog

> **Note**: This is a standalone version of CHANGELOG.md suitable for external distribution.
> For development context and complete technical history, see the main [CHANGELOG.md](CHANGELOG.md) file.

All notable changes to ThrottleBot are documented in this file.

---

## [1.1.0] - 2025-10-19

### Member Synchronization
Bot maintains accurate member records using 3-tier sync (initial, real-time, periodic 24h). Tracks status and admin roles. 3 new database tables added.

### Multi-Language Support
Complete localization in English, Russian, Dutch (135 messages each). New `/setlanguage` command. Per-group settings with smart English fallback.

### Private Chat Routing
All responses sent to private chat. Policy changes send dual messages (private + group). Automatic fallback to group @mention.

### Localized Notifications
Rate limit warnings at 95% threshold sent to private chat in user's language. Includes group context. Permanent history.

### Documentation Policy
Minimal documentation guidelines established. No code or database changes.

---

## [1.0.0] - 2025-10-18

### Multi-Window Rate Limiting
Complete rewrite with 3 independent windows and arbitrary durations (1-365 m/h/d).

**Added**: 3 windows (A/B/C), `/setwindow`, `/enablewindow`, `/disablewindow`, `/config` commands. ANY window triggers deletion. Automatic recovery. Override system with `/override <user> <whitelist|blacklist|clear> [duration]`. 99% storage reduction (14.6 GB → 3 MB per 1000 users).

**Changed**: 9 database tables (was 7). 12 commands total. Single binary deployment (38.4MB) with embedded migrations.

**Removed Legacy**: `/setlimit` → `/setwindow`, `/exempt` → `/override <user> whitelist`, `/restrict` → `/override <user> blacklist`, `/unrestrict` → `/override <user> clear`.

**Database**: Backup first, auto-migrates on startup.

---

## Database Schema

9 tables: users, groups, window_slots, simple_messages, user_overrides, schema_migrations, group_memberships, sync_metadata, sync_events.

## Migration Guide

From v0.x: Backup database first, auto-migrates on startup. Command changes: `/setlimit 1000 hour` → `/setwindow a 1000 1h && /enablewindow a`. `/exempt @user` → `/override @user whitelist`.

## Commands (12)

**User**: `/help`, `/mystatus`, `/config`

**Admin**: `/setlanguage`, `/setwindow`, `/enablewindow`, `/disablewindow`, `/override`, `/overrides`, `/checkuser`, `/pause`, `/resume`
