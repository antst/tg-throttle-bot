# ThrottleBot - Deployment Guide

> **Note**: This is a standalone version of DEPLOYMENT.md suitable for external distribution.
> For development procedures, testing guides, and complete technical details, see the main [DEPLOYMENT.md](DEPLOYMENT.md) file.

**Version**: 1.1.0  
**Date**: October 20, 2025  
**Status**: Production Ready ✅

## Overview

Production-ready Telegram bot that prevents group chat flooding with multi-window rate limiting, multi-language support, and private chat routing.

## Features

✅ **Multi-Window Rate Limiting**: 3 independent windows with arbitrary durations  
✅ **Multi-Language**: English, Russian, Dutch (135 messages)  
✅ **Private Chat Routing**: All responses to DM, zero group clutter  
✅ **Localized Notifications**: Warnings in user's language  
✅ **Member Sync**: 3-tier architecture (initial/realtime/periodic)  
✅ **Override System**: Whitelist/blacklist with expiration  
✅ **Production Ready**: 38.4MB Docker image, embedded migrations

## Prerequisites

**Required**:
- PostgreSQL 15+
- Telegram Bot Token from [@BotFather](https://t.me/botfather)
- Bot admin permissions: `can_restrict_members`, `can_delete_messages`, `can_read_messages`

**Recommended**:
- Docker & Docker Compose
- Kubernetes (production)
- Prometheus/Grafana (monitoring)

## Quick Start

### Docker Deployment

```bash
# Create environment file
cat > .env << EOF
TELEGRAM_TOKEN=your_bot_token_here
DATABASE_URL=postgres://throttlebot:password@db:5432/throttlebot?sslmode=disable
LOG_LEVEL=info
EOF

# Start services
docker-compose up -d

# Check status
docker-compose ps
docker-compose logs -f bot
```

### Kubernetes Deployment

```bash
# Create namespace
kubectl create namespace throttlebot

# Apply configurations
kubectl apply -f deployments/k8s/secret.yaml -n throttlebot
kubectl apply -f deployments/k8s/configmap.yaml -n throttlebot
kubectl apply -f deployments/k8s/deployment.yaml -n throttlebot

# Check status
kubectl get pods -n throttlebot
kubectl logs -f deployment/throttlebot -n throttlebot
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TELEGRAM_TOKEN` | Bot API token | Required |
| `DATABASE_URL` | PostgreSQL connection | Required |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `PORT` | Health check port | `8080` |

### Database Connection

```
postgres://username:password@hostname:5432/throttlebot?sslmode=require
```

**Production**: Use `sslmode=require`  
**Development**: Can use `sslmode=disable`

## Bot Setup

### 1. Create Bot

```
1. Message @BotFather on Telegram
2. Send /newbot
3. Follow instructions
4. Save bot token
```

### 2. Configure Bot

```
/setname - Throttle Bot
/setdescription - Prevents message flooding in groups
/setabouttext - Rate limiting bot for Telegram groups
```

### 3. Add to Group

```
1. Add bot to group
2. Promote to admin:
   ✅ Delete messages
   ✅ Restrict members
   ✅ Read messages
3. Send test message
```

### 4. Initial Configuration

```bash
# Set language
/setlanguage english

# Configure windows
/setwindow a 1000 30m
/enablewindow a

/setwindow b 5000 2h
/enablewindow b

/setwindow c 50000 7d
/enablewindow c
```

## Monitoring

### Health Endpoints

```bash
# Combined health
curl http://localhost:8080/health

# Liveness probe
curl http://localhost:8080/health/live

# Readiness probe (checks database)
curl http://localhost:8080/health/ready
```

### Database Monitoring

```sql
-- Active overrides
SELECT COUNT(*) FROM user_overrides 
WHERE expires_at IS NULL OR expires_at > NOW();

-- Recent messages
SELECT COUNT(*) FROM simple_messages 
WHERE timestamp > NOW() - INTERVAL '1 hour';

-- Member sync status
SELECT chat_id, status, total_members, last_sync_at 
FROM sync_metadata 
ORDER BY last_sync_at DESC;

-- Verify all 9 tables exist
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
ORDER BY table_name;
```

### Performance Metrics

- Message processing: <100ms average
- Database queries: <20ms per query
- Memory: <50MB per 1000 users
- Docker image: 38.4MB
- Storage: 3 KB per user (1-year window)

## Troubleshooting

### Bot Not Responding

```bash
# Check container
docker ps
docker logs throttlebot-bot

# Test token
curl https://api.telegram.org/bot<TOKEN>/getMe
```

### Database Connection

```bash
# Test connection
psql "postgres://user:password@localhost:5432/throttlebot"

# Check connections
SELECT count(*) FROM pg_stat_activity WHERE datname = 'throttlebot';
```

### Permission Issues

```
1. Verify bot is admin in group
2. Check permissions (delete, restrict, read)
3. Remove and re-add bot if needed
```

### Rate Limiting Not Working

```sql
-- Check group config exists
SELECT * FROM groups WHERE chat_id = YOUR_CHAT_ID;

-- Check enabled windows
SELECT * FROM window_slots WHERE chat_id = YOUR_CHAT_ID AND enabled = true;

-- Verify messages being tracked
SELECT COUNT(*) FROM simple_messages WHERE chat_id = YOUR_CHAT_ID;
```

## Backup & Recovery

### Database Backup

```bash
# Backup
pg_dump -h localhost -U throttlebot throttlebot > backup_$(date +%Y%m%d).sql

# Restore
psql -h localhost -U throttlebot throttlebot < backup_20251020.sql
```

### Critical Tables (Priority Order)

1. `groups` - Group configuration
2. `window_slots` - Rate limit settings
3. `user_overrides` - Exemptions
4. `sync_metadata` - Sync state
5. `group_memberships` - Member records
6. `users` - User records
7. `simple_messages` - Message log (lower priority)
8. `sync_events` - Audit trail (lower priority)
9. `schema_migrations` - Auto-managed

## Scaling

### Current Capacity

- Groups: 100+
- Users: 1000+ per group
- Messages: 1000+/minute
- Database: ~3 MB per 1000 users

### When to Scale

- Groups > 500
- Messages > 5000/minute
- Database > 1GB
- Response times > 500ms

### Scaling Options

1. **Horizontal**: Multiple bot instances (needs coordination)
2. **Database**: Read replicas for queries
3. **Caching**: Redis for frequent data
4. **Queue**: Message queue for high volume

## Security

- Bot runs as non-root (UID 1000)
- Read-only root filesystem
- SSL database connections (`sslmode=require`)
- Secrets via environment variables
- Input validation on all commands
- No sensitive data in logs

## Maintenance

### Regular Tasks

**Daily**:
- Check bot status
- Review error logs
- Monitor database size

**Weekly**:
- Database backup
- Review member sync metrics
- Check override expiration

**Monthly**:
- Rotate credentials
- Update Docker images
- Review scaling needs

### Log Monitoring

```bash
# View logs
docker-compose logs -f bot

# Filter errors
docker-compose logs bot | grep ERROR

# Check recent activity
docker-compose logs --tail=100 bot
```

## Support

- **Documentation**: README.md, CHANGELOG.md
- **Issues**: GitHub Issues
- **Telegram Bot API**: https://core.telegram.org/bots/api
- **PostgreSQL**: https://www.postgresql.org/

## Commands Reference

See [README.md](README.md) for complete command list (12 commands).

## Database Schema

See [README.md](README.md) for complete schema (9 tables).
