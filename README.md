# ThrottleBot - Telegram Message Rate Limiter

> **Note**: This is a standalone version of README.md suitable for external distribution.
> For development context, testing procedures, and complete documentation, see the main [README.md](README.md) file.

A production-ready Telegram bot that prevents chat flooding by enforcing configurable character rate limits per user using multi-window sliding window algorithms.

## ✨ Features

### Multi-Language Support
- **3 Languages**: English, Russian, Dutch (135 messages each)
- **Per-Group Settings**: Each group sets its own language
- **Private Chat Localization**: User's personal language for command responses
- **Group Notifications**: Group language for policy changes

### Private Chat Command Routing
- **All Responses Private**: Commands respond in user's DM
- **Zero Group Clutter**: Read-only commands stay silent
- **Policy Change Transparency**: Dual messages (private + group notification)
- **Automatic Fallback**: Uses group @mention if private unavailable

### Member Synchronization
- **3-Tier Architecture**: Initial + real-time + periodic (24h) sync
- **Accurate Records**: Current member list for all groups
- **Role Tracking**: Admin status changes tracked automatically
- **Status Monitoring**: Active/left/kicked member states

### Multi-Window Rate Limiting
- **3 Independent Windows**: Configure A, B, C with different limits/durations
- **Arbitrary Durations**: 1-365 units (m/h/d) - e.g., `30m`, `2h`, `7d`
- **ANY Window Triggers**: Message deleted when ANY window exceeded
- **Automatic Recovery**: Messages age out naturally

### Localized Notifications
- **Private Warnings**: 95% threshold to user's DM
- **Group Context**: Includes group name
- **Language-Aware**: User's personal language
- **Permanent History**: No auto-delete

### Override System
- **3-State Model**: Normal, whitelist, blacklist
- **Unified Command**: `/override` for all exemptions
- **Temporary & Permanent**: Optional expiration
- **@username Support**: Automatic harvesting

### Production Ready
- **Single Binary**: Embedded migrations, 38.4MB Docker image
- **Type-Safe Database**: SQLC-generated queries
- **9-Table Schema**: Optimized for performance
- **99% Storage Reduction**: 14.6 GB → 3 MB per 1000 users

## 🚀 Quick Start

### Prerequisites

- Go 1.25+ (for building from source)
- PostgreSQL 15+
- Telegram Bot Token from [@BotFather](https://t.me/botfather)

### Docker Deployment (Recommended)

```bash
# Pull latest image
docker pull ghcr.io/antst/tg-throttle-bot:latest

# Create .env file
cat > .env << EOF
TELEGRAM_TOKEN=your_bot_token_here
DATABASE_URL=postgres://user:password@db:5432/throttlebot?sslmode=disable
LOG_LEVEL=info
EOF

# Run with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f bot
```

### Kubernetes Deployment

```bash
kubectl create namespace throttlebot
kubectl apply -f deployments/k8s/secret.yaml -n throttlebot
kubectl apply -f deployments/k8s/configmap.yaml -n throttlebot
kubectl apply -f deployments/k8s/deployment.yaml -n throttlebot
kubectl apply -f deployments/k8s/service.yaml -n throttlebot
```

## 🤖 Bot Commands

### User Commands
- `/help` - Show available commands
- `/mystatus` - Check usage across all windows
- `/mygroups` - List groups where you and bot are members (with your role)
- `/config` - Show rate limit configuration

### Admin Commands

**Language Settings**:
- `/setlanguage <language>` - Set group language (en/ru/nl)

**Window Configuration**:
- `/setwindow <a|b|c> <limit> <duration>` - Configure window
  - Example: `/setwindow a 1000 30m` (1000 chars per 30 minutes)
- `/enablewindow <a|b|c>` - Enable window
- `/disablewindow <a|b|c>` - Disable window

**User Management**:
- `/override <user> <mode> [duration]` - Manage overrides
  - Modes: whitelist, blacklist, clear
  - Example: `/override @username whitelist 7d`
- `/overrides` - List all overrides
- `/checkuser <user>` - Check user usage

**Group Control**:
- `/pause [duration]` - Pause rate limiting
- `/resume` - Resume rate limiting

## 📊 Rate Limiting Behavior

### Multi-Window Architecture

Configure 3 independent windows (A, B, C) with different limits:
- **Window A**: Short-term burst (e.g., 1000 chars / 30 minutes)
- **Window B**: Mid-term sustained (e.g., 5000 chars / 2 hours)
- **Window C**: Long-term daily (e.g., 50000 chars / 7 days)

**Enforcement**: Messages deleted when ANY enabled window exceeded.

### Character Counting
- **Text**: UTF-8 character count
- **Media**: Caption + 50 base
- **Stickers/GIFs**: 50 each
- **Commands**: Excluded

### Warnings
- **95% threshold**: Single warning to private chat
- **100% threshold**: Message deleted, notification sent
- Includes group name context
- Uses user's language
- Permanent history

### Override System
- **Normal**: Follows all rate limits
- **Whitelist**: Bypasses ALL windows
- **Blacklist**: All messages deleted

**Priority**: Group Pause → User Override → Rate Limiter

### Automatic Recovery
- Messages age out naturally
- Real-time recalculation
- Instant recovery when usage drops

## 🗄️ Database Schema

9 optimized tables:
1. **users** - User records with language preferences
2. **groups** - Group configuration (pause, language)
3. **window_slots** - Rate limit config (3 per group)
4. **simple_messages** - Message log (shared)
5. **user_overrides** - Exemptions (whitelist/blacklist)
6. **schema_migrations** - Migration tracking
7. **group_memberships** - Member records (status, role)
8. **sync_metadata** - Sync state per group
9. **sync_events** - Audit trail

**Storage**: 3 KB per user (1-year window), 99% optimization

## ⚡ Performance

- Command response: <100ms average
- Database queries: <20ms
- Docker image: 38.4MB
- Memory: <50MB per 1000 users
- Throughput: 1000+ messages/minute

## 🔄 Background Workers

- **Override Cleanup**: Every 5 minutes (removes expired overrides)
- **Member Sync**: Every 24 hours (periodic group sync)

## Bot Setup

### 1. Create Bot
1. Message [@BotFather](https://t.me/botfather)
2. Send `/newbot` and follow instructions
3. Save the bot token

### 2. Add to Group
1. Add bot to your group
2. Promote to admin:
   - ✅ Delete messages
   - ✅ Restrict members
3. Send a test message to activate

### 3. Configure
```bash
# Set language
/setlanguage english

# Configure windows
/setwindow a 1000 30m
/enablewindow a

/setwindow b 5000 2h
/enablewindow b
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `TELEGRAM_TOKEN` | Bot API token from @BotFather | ✅ Yes |
| `DATABASE_URL` | PostgreSQL connection string | ✅ Yes |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | No |
| `PORT` | Health check HTTP port | No |

## Health Checks

- `GET /health` - Combined health check
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe (checks database)

## Security

- Non-root user (UID 1000)
- Read-only root filesystem
- SSL database connections in production
- Input validation on all commands
- Rate limiting for Telegram API

## Scaling

**Current Capacity**:
- Groups: 100+ simultaneous
- Users: 1000+ per group
- Messages: 1000+/minute
- Database: ~3 MB per 1000 users

**When to Scale**:
- Groups > 500
- Messages > 5000/minute
- Database > 1GB
- Response times > 500ms

## Support

- Issues: GitHub Issues
- Documentation: See [DEPLOYMENT.md](DEPLOYMENT.md)
- Changelog: See [CHANGELOG.md](CHANGELOG.md)

## License

[LICENSE](LICENSE)
