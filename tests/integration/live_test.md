# Live Integration Tests - Private Chat Replies (Feature 008)

**Date**: 2025-10-19  
**Feature**: Private Chat Replies with Group Notifications  
**Method**: Manual testing with docker-compose

## Prerequisites

- Docker and docker-compose installed
- `.env` file configured with Telegram bot token
- Bot added to test group (e.g., "home")
- Telegram client (mobile or desktop) logged in

## Setup

```bash
cd /root/tg-throttle-bot
docker-compose up --build
```

Expected output:
```
bot_1  | INFO  Bot started successfully
bot_1  | INFO  Connected to database
bot_1  | INFO  Listening for updates...
```

---

## User Story 1: Private Chat Responses

### Test 1.1: Read-Only Command Private Routing

**Objective**: Verify `/config` responds in private chat, not group

**Steps**:
1. Open Telegram, navigate to your test group
2. Send command: `/config`
3. Check private chat with the bot
4. Check the group chat

**Expected Results**:
- ✅ Private chat: Displays group configuration (windows, language, etc.)
- ✅ Group chat: NO message from bot (silent)

**Pass/Fail**: ________

---

### Test 1.2: Help Command Private Routing

**Objective**: Verify `/help` responds in private chat

**Steps**:
1. In test group, send: `/help`
2. Check private chat with bot
3. Check group chat

**Expected Results**:
- ✅ Private chat: Help text with command list
- ✅ Group chat: NO message from bot

**Pass/Fail**: ________

---

### Test 1.3: Fallback to Group @Mention

**Objective**: Verify fallback when user blocks bot

**Steps**:
1. In Telegram Settings → Privacy & Security → Blocked users
2. Block the bot
3. In test group, send: `/config`
4. Check group chat

**Expected Results**:
- ✅ Group chat shows message: `@yourusername [configuration text]\n\n💬 Tip: Start a private chat with me to receive responses privately.`
- ✅ Message includes @mention of your username
- ✅ Message includes instruction to start private chat

**Steps to Reset**:
1. Unblock the bot in Telegram settings
2. Send `/start` in private chat with bot
3. Send `/config` in group again
4. Verify it goes back to private chat

**Pass/Fail**: ________

---

## User Story 2: Dual Messages for Policy Changes

### Test 2.1: SetWindow Dual Message

**Objective**: Verify policy command sends both private + group messages

**Steps**:
1. Ensure bot is NOT blocked (unblock if needed)
2. In test group, send: `/setwindow a 5000 1h`
3. Check private chat
4. Check group chat
5. Note the timestamps

**Expected Results**:
- ✅ Private chat: "✅ Window A configured: 5000 characters per 1 hour"
- ✅ Group chat: "Window A updated: 5000 chars/1h by @yourusername"
- ✅ Both messages arrive within 2 seconds
- ✅ Group notification is concise (one line)

**Pass/Fail**: ________

---

### Test 2.2: EnableWindow Dual Message

**Objective**: Verify enable command sends dual messages

**Steps**:
1. In test group, send: `/enablewindow b`
2. Check private chat
3. Check group chat

**Expected Results**:
- ✅ Private chat: "✅ Window B enabled"
- ✅ Group chat: "Window B enabled by @yourusername"

**Pass/Fail**: ________

---

### Test 2.3: DisableWindow Dual Message

**Objective**: Verify disable command sends dual messages

**Steps**:
1. In test group, send: `/disablewindow c`
2. Check private chat
3. Check group chat

**Expected Results**:
- ✅ Private chat: "✅ Window C disabled"
- ✅ Group chat: "Window C disabled by @yourusername"

**Pass/Fail**: ________

---

### Test 2.4: SetLanguage Dual Message

**Objective**: Verify language change sends dual messages

**Steps**:
1. Note current group language (check with `/config`)
2. In test group, send: `/setlanguage ru` (or `en` if already Russian)
3. Check private chat
4. Check group chat

**Expected Results**:
- ✅ Private chat: Language change confirmation (in your user language)
- ✅ Group chat: "Group language changed to [language] by @yourusername"

**Pass/Fail**: ________

---

## User Story 2: Localization

### Test 2.5: User Language vs Group Language

**Objective**: Verify private uses user language, group uses group language

**Setup**:
1. Set group language to Russian: `/setlanguage ru`
2. Verify with `/config` - should show Russian interface

**Steps**:
1. In test group, send: `/setwindow a 4000 2h`
2. Check private chat message language
3. Check group chat message language

**Expected Results**:
- ✅ Private chat: Uses your user language preference (may be English if not set)
- ✅ Group chat: Uses Russian (group's language)
- ✅ Both messages contain correct window information

**Cleanup**:
1. Reset group language: `/setlanguage en`

**Pass/Fail**: ________

---

## User Story 2: Fallback with Policy Commands

### Test 2.6: Dual Message When Private Chat Blocked

**Objective**: Verify policy command still sends group notification when private fails

**Steps**:
1. Block the bot (Settings → Privacy → Blocked users)
2. In test group, send: `/setwindow a 3000 30m`
3. Check group chat (should see 2 messages)

**Expected Results**:
- ✅ Group chat message 1 (fallback): `@yourusername ✅ Window A configured...` + tip
- ✅ Group chat message 2 (notification): `Window A updated: 3000 chars/30m by @yourusername`
- ✅ Both messages appear (group notification still sent even if private failed)

**Cleanup**:
1. Unblock the bot
2. Send `/start` in private chat

**Pass/Fail**: ________

---

## Error Handling Tests

### Test 3.1: Invalid Command

**Steps**:
1. In test group, send: `/invalidcommand`
2. Check private chat

**Expected Results**:
- ✅ Private chat: "❌ Unknown command: invalidcommand\nUse /help to see available commands"
- ✅ Group chat: Silent

**Pass/Fail**: ________

---

### Test 3.2: Permission Denied

**Steps**:
1. Ask a NON-ADMIN group member to send: `/setwindow a 1000 1h`
2. Check their private chat

**Expected Results**:
- ✅ Private chat: "This command is only available to group administrators"
- ✅ Group chat: Silent

**Pass/Fail**: ________

---

## Performance Tests

### Test 4.1: Response Time

**Objective**: Measure response time for private routing

**Steps**:
1. In test group, send: `/config`
2. Note timestamp in group
3. Note timestamp of response in private chat
4. Calculate difference

**Expected Results**:
- ✅ Response arrives in < 500ms (command processing)
- ✅ Total delivery time < 2 seconds

**Actual Time**: ________

**Pass/Fail**: ________

---

### Test 4.2: Concurrent Commands

**Objective**: Verify bot handles multiple simultaneous commands

**Steps**:
1. Quickly send multiple commands:
   - `/config`
   - `/help`
   - `/setwindow a 1000 1h`
2. Check private chat

**Expected Results**:
- ✅ All 3 responses arrive in private chat
- ✅ Responses arrive in correct order
- ✅ No errors or missing messages

**Pass/Fail**: ________

---

## Test Summary

| Test ID | Test Name | Pass/Fail | Notes |
|---------|-----------|-----------|-------|
| 1.1 | Read-Only Private Routing | | |
| 1.2 | Help Command Private | | |
| 1.3 | Fallback to Group | | |
| 2.1 | SetWindow Dual Message | | |
| 2.2 | EnableWindow Dual Message | | |
| 2.3 | DisableWindow Dual Message | | |
| 2.4 | SetLanguage Dual Message | | |
| 2.5 | User vs Group Language | | |
| 2.6 | Dual Message with Blocked Bot | | |
| 3.1 | Invalid Command Error | | |
| 3.2 | Permission Denied | | |
| 4.1 | Response Time | | |
| 4.2 | Concurrent Commands | | |

**Overall Pass Rate**: _____ / 13

---

## Known Issues

_Document any bugs or unexpected behavior discovered during testing_

---

## Test Environment

- **Date**: __________
- **Docker Image**: (from docker-compose up)
- **Bot Version**: (from logs)
- **Telegram Client**: Mobile / Desktop / Web
- **Test Group ID**: __________
- **Tester Username**: @__________

---

## Sign-Off

**Tester Name**: __________  
**Date**: __________  
**Status**: PASS / FAIL / PARTIAL

**Notes**:
