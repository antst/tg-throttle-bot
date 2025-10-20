#!/usr/bin/env python3
"""
Generate REAL Russian and Dutch translations for all command keys.
Proper translations, not placeholders!
"""

import json

# REAL translations dictionary
TRANSLATIONS = {
    # Common errors
    "cmd_error_not_admin": {
        "ru": "❌ Вы должны быть администратором, чтобы {{.action}}.",
        "nl": "❌ Je moet een beheerder zijn om {{.action}}."
    },
    "cmd_error_invalid_user_id": {
        "ru": "❌ Неверный ID пользователя",
        "nl": "❌ Ongeldige gebruikers-ID"
    },
    "cmd_error_database_error": {
        "ru": "❌ Ошибка базы данных. Пожалуйста, попробуйте снова.",
        "nl": "❌ Databasefout. Probeer het opnieuw."
    },
    "cmd_error_no_rate_limiting": {
        "ru": "📊 Для этой группы ещё не настроено ограничение скорости.",
        "nl": "📊 Nog geen snelheidsbeperking geconfigureerd voor deze groep."
    },
    "cmd_error_invalid_language": {
        "ru": "❌ Неверный язык. Поддерживаются: en (English), ru (Русский), nl (Nederlands)",
        "nl": "❌ Ongeldige taal. Ondersteund: en (English), ru (Русский), nl (Nederlands)"
    },
    
    # /setwindow command
    "cmd_setwindow_usage_private": {
        "ru": "❌ Использование из личного чата: /setwindow @имягруппы <a|b|c> <лимит> <длительность>\nПример: /setwindow @mygroup a 1000 30m",
        "nl": "❌ Gebruik vanuit privéchat: /setwindow @groepnaam <a|b|c> <limiet> <duur>\nVoorbeeld: /setwindow @mygroup a 1000 30m"
    },
    "cmd_setwindow_usage_group": {
        "ru": "❌ Использование: /setwindow <a|b|c> <лимит> <длительность>\nПримеры:\n  /setwindow a 1000 30m\n  /setwindow b 5000 2h\n  /setwindow c 10000 7d",
        "nl": "❌ Gebruik: /setwindow <a|b|c> <limiet> <duur>\nVoorbeelden:\n  /setwindow a 1000 30m\n  /setwindow b 5000 2h\n  /setwindow c 10000 7d"
    },
    "cmd_setwindow_success": {
        "ru": "✅ Окно {{.window}} настроено:\n  • Лимит: {{.limit}} символов\n  • Длительность: {{.duration}}\n  • Статус: {{.status}}",
        "nl": "✅ Venster {{.window}} geconfigureerd:\n  • Limiet: {{.limit}} tekens\n  • Duur: {{.duration}}\n  • Status: {{.status}}"
    },
    "cmd_setwindow_invalid_slot": {
        "ru": "❌ Неверный слот окна '{{.slot}}'. Должно быть: a, b или c",
        "nl": "❌ Ongeldig vensterslot '{{.slot}}'. Moet zijn: a, b of c"
    },
    "cmd_setwindow_invalid_limit": {
        "ru": "❌ Неверный лимит символов: {{.error}}",
        "nl": "❌ Ongeldige tekenlimiet: {{.error}}"
    },
    "cmd_setwindow_invalid_duration": {
        "ru": "❌ Неверная длительность: {{.error}}",
        "nl": "❌ Ongeldige duur: {{.error}}"
    },
    
    # /config command
    "cmd_config_usage_private": {
        "ru": "❌ Использование из личного чата: /config @имягруппы",
        "nl": "❌ Gebruik vanuit privéchat: /config @groepnaam"
    },
    "cmd_config_no_limits": {
        "ru": "📊 Для этой группы ещё не настроено ограничение скорости.\nАдминистратор может использовать /setwindow для настройки окон.",
        "nl": "📊 Nog geen snelheidsbeperking geconfigureerd voor deze groep.\nBeheerder kan /setwindow gebruiken om vensters te configureren."
    },
    "cmd_config_title": {
        "ru": "📊 **Конфигурация ограничения скорости**\n\nГруппа: {{.group}}",
        "nl": "📊 **Snelheidsbeperkingsconfiguratie**\n\nGroep: {{.group}}"
    },
    "cmd_config_paused": {
        "ru": "\n⏸ **Статус**: Приостановлено{{.until}}",
        "nl": "\n⏸ **Status**: Gepauzeerd{{.until}}"
    },
    "cmd_config_active": {
        "ru": "\n▶️ **Статус**: Активно",
        "nl": "\n▶️ **Status**: Actief"
    },
    "cmd_config_window_enabled": {
        "ru": "\n\n✅ **Окно {{.slot}}** (включено)\n  • Лимит: {{.limit}} символов\n  • Длительность: {{.duration}}",
        "nl": "\n\n✅ **Venster {{.slot}}** (ingeschakeld)\n  • Limiet: {{.limit}} tekens\n  • Duur: {{.duration}}"
    },
    "cmd_config_window_disabled": {
        "ru": "\n\n🚫 **Окно {{.slot}}** (отключено)\n  • Лимит: {{.limit}} символов\n  • Длительность: {{.duration}}",
        "nl": "\n\n🚫 **Venster {{.slot}}** (uitgeschakeld)\n  • Limiet: {{.limit}} tekens\n  • Duur: {{.duration}}"
    },
    "cmd_config_usage_hint": {
        "ru": "\n\n💡 Используйте /enablewindow или /disablewindow для переключения окон.",
        "nl": "\n\n💡 Gebruik /enablewindow of /disablewindow om vensters te schakelen."
    },
    
    # /enablewindow command
    "cmd_enablewindow_usage_private": {
        "ru": "❌ Использование из личного чата: /enablewindow @имягруппы <a|b|c>",
        "nl": "❌ Gebruik vanuit privéchat: /enablewindow @groepnaam <a|b|c>"
    },
    "cmd_enablewindow_usage_group": {
        "ru": "❌ Использование: /enablewindow <a|b|c>",
        "nl": "❌ Gebruik: /enablewindow <a|b|c>"
    },
    "cmd_enablewindow_success": {
        "ru": "✅ Окно {{.window}} включено.",
        "nl": "✅ Venster {{.window}} ingeschakeld."
    },
    "cmd_enablewindow_invalid_slot": {
        "ru": "❌ Неверный слот окна '{{.slot}}'. Должно быть: a, b или c",
        "nl": "❌ Ongeldig vensterslot '{{.slot}}'. Moet zijn: a, b of c"
    },
    "cmd_enablewindow_not_configured": {
        "ru": "❌ Окно {{.window}} ещё не настроено. Сначала используйте /setwindow.",
        "nl": "❌ Venster {{.window}} is nog niet geconfigureerd. Gebruik eerst /setwindow."
    },
    
    # /disablewindow command
    "cmd_disablewindow_usage_private": {
        "ru": "❌ Использование из личного чата: /disablewindow @имягруппы <a|b|c>",
        "nl": "❌ Gebruik vanuit privéchat: /disablewindow @groepnaam <a|b|c>"
    },
    "cmd_disablewindow_usage_group": {
        "ru": "❌ Использование: /disablewindow <a|b|c>",
        "nl": "❌ Gebruik: /disablewindow <a|b|c>"
    },
    "cmd_disablewindow_success": {
        "ru": "✅ Окно {{.window}} отключено.",
        "nl": "✅ Venster {{.window}} uitgeschakeld."
    },
    "cmd_disablewindow_invalid_slot": {
        "ru": "❌ Неверный слот окна '{{.slot}}'. Должно быть: a, b или c",
        "nl": "❌ Ongeldig vensterslot '{{.slot}}'. Moet zijn: a, b of c"
    },
    "cmd_disablewindow_not_configured": {
        "ru": "❌ Окно {{.window}} ещё не настроено.",
        "nl": "❌ Venster {{.window}} is nog niet geconfigureerd."
    },
    
    # /override command
    "cmd_override_usage_private": {
        "ru": "❌ Использование из личного чата: /override @имягруппы <user_id|@username> <режим> [длительность]\nРежимы: whitelist, blacklist, clear",
        "nl": "❌ Gebruik vanuit privéchat: /override @groepnaam <user_id|@username> <modus> [duur]\nModi: whitelist, blacklist, clear"
    },
    "cmd_override_usage_group": {
        "ru": "❌ Использование: /override <user_id|@username> <режим> [длительность]\nРежимы: whitelist, blacklist, clear\nПримеры:\n  /override @user whitelist\n  /override 12345 blacklist 7d\n  /override @user clear",
        "nl": "❌ Gebruik: /override <user_id|@username> <modus> [duur]\nModi: whitelist, blacklist, clear\nVoorbeelden:\n  /override @user whitelist\n  /override 12345 blacklist 7d\n  /override @user clear"
    },
    "cmd_override_invalid_mode": {
        "ru": "❌ Неверный режим '{{.mode}}'. Должно быть: whitelist, blacklist или clear",
        "nl": "❌ Ongeldige modus '{{.mode}}'. Moet zijn: whitelist, blacklist of clear"
    },
    "cmd_override_invalid_duration": {
        "ru": "❌ Неверная длительность: {{.error}}",
        "nl": "❌ Ongeldige duur: {{.error}}"
    },
    "cmd_override_whitelist_success": {
        "ru": "✅ Пользователь {{.user}} добавлен в белый список{{.duration}}.",
        "nl": "✅ Gebruiker {{.user}} op witte lijst gezet{{.duration}}."
    },
    "cmd_override_blacklist_success": {
        "ru": "✅ Пользователь {{.user}} добавлен в черный список{{.duration}}.",
        "nl": "✅ Gebruiker {{.user}} op zwarte lijst gezet{{.duration}}."
    },
    "cmd_override_cleared": {
        "ru": "✅ Переопределение удалено для пользователя {{.user}}.",
        "nl": "✅ Overschrijving verwijderd voor gebruiker {{.user}}."
    },
    
    # /checkuser command
    "cmd_checkuser_usage_private": {
        "ru": "❌ Использование из личного чата: /checkuser @имягруппы <user_id|@username>",
        "nl": "❌ Gebruik vanuit privéchat: /checkuser @groepnaam <user_id|@username>"
    },
    "cmd_checkuser_usage_group": {
        "ru": "❌ Использование: /checkuser <user_id|@username>",
        "nl": "❌ Gebruik: /checkuser <user_id|@username>"
    },
    "cmd_checkuser_no_config": {
        "ru": "📊 Для этой группы ещё не настроено ограничение скорости.",
        "nl": "📊 Nog geen snelheidsbeperking geconfigureerd voor deze groep."
    },
    "cmd_checkuser_title": {
        "ru": "👤 **Статус пользователя: {{.user}}**\n\nГруппа: {{.group}}",
        "nl": "👤 **Gebruikersstatus: {{.user}}**\n\nGroep: {{.group}}"
    },
    "cmd_checkuser_override_whitelist": {
        "ru": "\n\n✅ **Ручное переопределение**: В белом списке (освобожден от всех лимитов){{.expires}}",
        "nl": "\n\n✅ **Handmatige overschrijving**: Op witte lijst (vrijgesteld van alle limieten){{.expires}}"
    },
    "cmd_checkuser_override_blacklist": {
        "ru": "\n\n🚫 **Ручное переопределение**: В черном списке (все сообщения удаляются){{.expires}}",
        "nl": "\n\n🚫 **Handmatige overschrijving**: Op zwarte lijst (alle berichten verwijderd){{.expires}}"
    },
    "cmd_checkuser_no_override": {
        "ru": "\n\nℹ️ Ручное переопределение не установлено.",
        "nl": "\n\nℹ️ Geen handmatige overschrijving ingesteld."
    },
    "cmd_checkuser_window_ok": {
        "ru": "\n\n✅ **Окно {{.slot}}**: {{.current}}/{{.limit}} симв. ({{.percentage}}%)",
        "nl": "\n\n✅ **Venster {{.slot}}**: {{.current}}/{{.limit}} tekens ({{.percentage}}%)"
    },
    "cmd_checkuser_window_warning": {
        "ru": "\n\n⚠️ **Окно {{.slot}}**: {{.current}}/{{.limit}} симв. ({{.percentage}}% - приближается к лимиту)",
        "nl": "\n\n⚠️ **Venster {{.slot}}**: {{.current}}/{{.limit}} tekens ({{.percentage}}% - nadert limiet)"
    },
    "cmd_checkuser_window_exceeded": {
        "ru": "\n\n❌ **Окно {{.slot}}**: {{.current}}/{{.limit}} симв. ({{.percentage}}% - ПРЕВЫШЕНО)",
        "nl": "\n\n❌ **Venster {{.slot}}**: {{.current}}/{{.limit}} tekens ({{.percentage}}% - OVERSCHREDEN)"
    },
    "cmd_checkuser_window_disabled": {
        "ru": "\n\n🚫 **Окно {{.slot}}**: Отключено",
        "nl": "\n\n🚫 **Venster {{.slot}}**: Uitgeschakeld"
    },
    
    # /showoverrides command
    "cmd_showoverrides_usage_private": {
        "ru": "❌ Использование из личного чата: /showoverrides @имягруппы",
        "nl": "❌ Gebruik vanuit privéchat: /showoverrides @groepnaam"
    },
    "cmd_showoverrides_no_overrides": {
        "ru": "📋 **Ручные переопределения**\n\nДля этой группы не установлено ручных переопределений.\n\n💡 Используйте /override <user_id|@username> <whitelist|blacklist|clear> для управления переопределениями.",
        "nl": "📋 **Handmatige overschrijvingen**\n\nGeen handmatige overschrijvingen ingesteld voor deze groep.\n\n💡 Gebruik /override <user_id|@username> <whitelist|blacklist|clear> om overschrijvingen te beheren."
    },
    "cmd_showoverrides_title": {
        "ru": "📋 **Ручные переопределения**\n\nГруппа: {{.group}}",
        "nl": "📋 **Handmatige overschrijvingen**\n\nGroep: {{.group}}"
    },
    "cmd_showoverrides_whitelist_entry": {
        "ru": "\n\n✅ {{.user}}\n  • Режим: В белом списке (освобожден от лимитов)\n  • Установлено: {{.created}}{{.expires}}",
        "nl": "\n\n✅ {{.user}}\n  • Modus: Op witte lijst (vrijgesteld van limieten)\n  • Ingesteld: {{.created}}{{.expires}}"
    },
    "cmd_showoverrides_blacklist_entry": {
        "ru": "\n\n🚫 {{.user}}\n  • Режим: В черном списке (все сообщения удаляются)\n  • Установлено: {{.created}}{{.expires}}",
        "nl": "\n\n🚫 {{.user}}\n  • Modus: Op zwarte lijst (alle berichten verwijderd)\n  • Ingesteld: {{.created}}{{.expires}}"
    },
    
    # /setlanguage command
    "cmd_setlanguage_usage_private_with_group": {
        "ru": "❌ Использование:\n  /setlanguage en  (устанавливает язык для личного чата)\n  /setlanguage @имягруппы en  (устанавливает язык для группы)",
        "nl": "❌ Gebruik:\n  /setlanguage en  (stelt taal in voor privéchat)\n  /setlanguage @groepnaam en  (stelt taal in voor groep)"
    },
    "cmd_setlanguage_usage": {
        "ru": "❌ Использование: /setlanguage <язык>\nПример: /setlanguage en",
        "nl": "❌ Gebruik: /setlanguage <taal>\nVoorbeeld: /setlanguage en"
    },
    "cmd_setlanguage_success_user": {
        "ru": "✅ Ваш язык установлен на: {{.language}}\n\nВсе ответы бота теперь будут на {{.language_name}}.",
        "nl": "✅ Je taal is ingesteld op: {{.language}}\n\nAlle botreacties zijn nu in {{.language_name}}."
    },
    "cmd_setlanguage_success_group": {
        "ru": "✅ Язык группы установлен на: {{.language}}\n\nВсе уведомления группы теперь будут на {{.language_name}}.",
        "nl": "✅ Groepstaal ingesteld op: {{.language}}\n\nAlle groepmeldingen zijn nu in {{.language_name}}."
    },
    
    # /pause command
    "cmd_pause_usage_private": {
        "ru": "❌ Использование из личного чата: /pause @имягруппы [длительность]",
        "nl": "❌ Gebruik vanuit privéchat: /pause @groepnaam [duur]"
    },
    "cmd_pause_usage_group": {
        "ru": "❌ Использование: /pause [длительность]\nПримеры:\n  /pause (приостановить бессрочно)\n  /pause 30m (приостановить на 30 минут)\n  /pause 2h (приостановить на 2 часа)",
        "nl": "❌ Gebruik: /pause [duur]\nVoorbeelden:\n  /pause (pauzeer voor onbepaalde tijd)\n  /pause 30m (pauzeer voor 30 minuten)\n  /pause 2h (pauzeer voor 2 uur)"
    },
    "cmd_pause_invalid_duration": {
        "ru": "❌ Неверная длительность: {{.error}}",
        "nl": "❌ Ongeldige duur: {{.error}}"
    },
    "cmd_pause_success_temporary": {
        "ru": "✅ Ограничение скорости приостановлено на {{.duration}}.\n\nВсе пользователи могут отправлять неограниченное количество сообщений до {{.until}}.\nОграничение скорости возобновится автоматически.",
        "nl": "✅ Snelheidsbeperking gepauzeerd voor {{.duration}}.\n\nAlle gebruikers kunnen onbeperkt berichten sturen tot {{.until}}.\nSnelheidsbeperking wordt automatisch hervat."
    },
    "cmd_pause_success_indefinite": {
        "ru": "✅ Ограничение скорости приостановлено бессрочно.\nВсе пользователи могут отправлять неограниченное количество сообщений, пока вы не используете /resume.",
        "nl": "✅ Snelheidsbeperking voor onbepaalde tijd gepauzeerd.\nAlle gebruikers kunnen onbeperkt berichten sturen totdat je /resume gebruikt."
    },
    
    # /resume command
    "cmd_resume_usage_private": {
        "ru": "❌ Использование из личного чата: /resume @имягруппы",
        "nl": "❌ Gebruik vanuit privéchat: /resume @groepnaam"
    },
    "cmd_resume_not_paused": {
        "ru": "ℹ️ Ограничение скорости не приостановлено для этой группы.",
        "nl": "ℹ️ Snelheidsbeperking is niet gepauzeerd voor deze groep."
    },
    "cmd_resume_success": {
        "ru": "✅ Ограничение скорости возобновлено.\nВсе включенные окна теперь применяют лимиты снова.",
        "nl": "✅ Snelheidsbeperking hervat.\nAlle ingeschakelde vensters handhaven nu weer limieten."
    },
    
    # /mystatus command
    "cmd_mystatus_usage_private": {
        "ru": "❌ Использование из личного чата: /mystatus @имягруппы",
        "nl": "❌ Gebruik vanuit privéchat: /mystatus @groepnaam"
    },
    "cmd_mystatus_no_config": {
        "ru": "📊 Для этой группы не настроено ограничение скорости.",
        "nl": "📊 Geen snelheidsbeperking geconfigureerd voor deze groep."
    },
    "cmd_mystatus_title": {
        "ru": "📊 **Ваш статус**\n\nГруппа: {{.group}}",
        "nl": "📊 **Je status**\n\nGroep: {{.group}}"
    },
    "cmd_mystatus_paused": {
        "ru": "\n\n⏸ Ограничение скорости в данный момент приостановлено{{.until}}.",
        "nl": "\n\n⏸ Snelheidsbeperking is momenteel gepauzeerd{{.until}}."
    },
    "cmd_mystatus_override_whitelist": {
        "ru": "\n\n✅ Вы в белом списке (освобождены от всех лимитов){{.expires}}.",
        "nl": "\n\n✅ Je staat op de witte lijst (vrijgesteld van alle limieten){{.expires}}."
    },
    "cmd_mystatus_override_blacklist": {
        "ru": "\n\n🚫 Вы в черном списке (все ваши сообщения удаляются){{.expires}}.",
        "nl": "\n\n🚫 Je staat op de zwarte lijst (al je berichten worden verwijderd){{.expires}}."
    },
    "cmd_mystatus_window_ok": {
        "ru": "\n\n✅ **Окно {{.slot}}**: {{.current}}/{{.limit}} симв. ({{.percentage}}%)",
        "nl": "\n\n✅ **Venster {{.slot}}**: {{.current}}/{{.limit}} tekens ({{.percentage}}%)"
    },
    "cmd_mystatus_window_warning": {
        "ru": "\n\n⚠️ **Окно {{.slot}}**: {{.current}}/{{.limit}} симв. ({{.percentage}}% - приближается к лимиту!)",
        "nl": "\n\n⚠️ **Venster {{.slot}}**: {{.current}}/{{.limit}} tekens ({{.percentage}}% - nadert limiet!)"
    },
    "cmd_mystatus_window_exceeded": {
        "ru": "\n\n❌ **Окно {{.slot}}**: {{.current}}/{{.limit}} симв. ({{.percentage}}% - лимит превышен!)",
        "nl": "\n\n❌ **Venster {{.slot}}**: {{.current}}/{{.limit}} tekens ({{.percentage}}% - limiet overschreden!)"
    },
    "cmd_mystatus_window_disabled": {
        "ru": "\n\n🚫 **Окно {{.slot}}**: Отключено",
        "nl": "\n\n🚫 **Venster {{.slot}}**: Uitgeschakeld"
    },
    "cmd_help_user": {
        "ru": "🤖 Throttle Bot - Многооконный ограничитель скорости\n\n👤 **Ваши команды:**\n• /help - Показать это справочное сообщение\n• /mystatus - Проверить текущее использование во всех окнах\n• /config - Показать конфигурацию ограничения скорости\n\n💡 **Как это работает:**\nКаждая группа имеет до 3 окон ограничения скорости (A, B, C) с разными:\n- Лимитами символов (например, 1000, 5000, 10000 символов)\n- Временными окнами (например, 30 минут, 2 часа, 7 дней)\n\nВаши сообщения учитываются во всех активных окнах. Если вы превысите лимит ЛЮБОГО окна, ваши сообщения будут автоматически удалены до сброса окна.\n\n🎯 **Состояния переопределения:**\n- **Обычное**: Применяется ограничение скорости (по умолчанию)\n- **Белый список**: Ваши сообщения всегда разрешены ✅\n- **Черный список**: Ваши сообщения всегда блокируются ❌\n\n💬 Свяжитесь с администратором, если вам нужна помощь!",
        "nl": "🤖 Throttle Bot - Multi-venster snelheidsbegrenzer\n\n👤 **Jouw commando's:**\n• /help - Toon dit helpbericht\n• /mystatus - Controleer je huidige gebruik in alle vensters\n• /config - Toon snelheidsbeperkingsconfiguratie\n\n💡 **Hoe het werkt:**\nElke groep heeft maximaal 3 snelheidsbeperkingsvensters (A, B, C) met verschillende:\n- Tekenlimieten (bijv. 1000, 5000, 10000 tekens)\n- Tijdvensters (bijv. 30 minuten, 2 uur, 7 dagen)\n\nJe berichten tellen mee voor alle actieve vensters. Als je de limiet van ELK venster overschrijdt, worden je berichten automatisch verwijderd totdat het venster wordt gereset.\n\n🎯 **Overschrijvingsstatussen:**\n- **Normaal**: Snelheidsbeperking is van toepassing (standaard)\n- **Witte lijst**: Je berichten zijn altijd toegestaan ✅\n- **Zwarte lijst**: Je berichten worden altijd geblokkeerd ❌\n\n💬 Neem contact op met een beheerder als je hulp nodig hebt!"
    },
    "cmd_help_admin": {
        "ru": "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n👑 **КОМАНДЫ АДМИНИСТРАТОРА**\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n📝 **Конфигурация окна:**\n• /setwindow <a|b|c> <лимит> <длительность> - Настроить окно\n  Пример: /setwindow a 1000 30m\n• /enablewindow <a|b|c> - Включить окно\n• /disablewindow <a|b|c> - Отключить окно\n\n🔧 **Управление пользователями:**\n• /checkuser <user_id|@username> - Проверить использование конкретного пользователя\n• /overrides - Список всех пользователей с ручными переопределениями\n• /override <user_id|@username> <режим> [длительность] - Управление переопределениями пользователей\n  **Режимы**: whitelist | blacklist | clear\n  **Примеры**:\n    /override 123456789 whitelist - Обход всего ограничения скорости\n    /override @testuser whitelist 7d - Белый список по имени пользователя (7 дней)\n    /override 123456789 blacklist 2h - Блокировать все сообщения (2 часа)\n    /override @testuser clear - Вернуться к нормальному\n\n⏸️ **Управление группой:**\n• /pause [длительность] - Приостановить ВСЕ ограничения скорости (опционально: 30m, 2h, 7d)\n• /resume - Возобновить ограничение скорости\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
        "nl": "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n👑 **BEHEERDER COMMANDO'S**\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n📝 **Vensterconfiguratie:**\n• /setwindow <a|b|c> <limiet> <duur> - Venster configureren\n  Voorbeeld: /setwindow a 1000 30m\n• /enablewindow <a|b|c> - Venster inschakelen\n• /disablewindow <a|b|c> - Venster uitschakelen\n\n🔧 **Gebruikersbeheer:**\n• /checkuser <user_id|@username> - Specifiek gebruikersverbruik controleren\n• /overrides - Lijst alle gebruikers met handmatige overschrijvingen\n• /override <user_id|@username> <modus> [duur] - Gebruikersoverschrijvingen beheren\n  **Modi**: whitelist | blacklist | clear\n  **Voorbeelden**:\n    /override 123456789 whitelist - Omzeil alle snelheidsbeperking\n    /override @testuser whitelist 7d - Witte lijst op gebruikersnaam (7 dagen)\n    /override 123456789 blacklist 2h - Blokkeer alle berichten (2 uur)\n    /override @testuser clear - Terug naar normaal\n\n⏸️ **Groepsbesturing:**\n• /pause [duur] - Pauzeer ALLE snelheidsbeperking (optioneel: 30m, 2h, 7d)\n• /resume - Hervat snelheidsbeperking\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    }
}

# Load files
with open('internal/i18n/translations/en.json', 'r', encoding='utf-8') as f:
    en_data = json.load(f)

with open('internal/i18n/translations/ru.json', 'r', encoding='utf-8') as f:
    ru_data = json.load(f)

with open('internal/i18n/translations/nl.json', 'r', encoding='utf-8') as f:
    nl_data = json.load(f)

# Apply real translations
count_ru = 0
count_nl = 0

for key, translations in TRANSLATIONS.items():
    if key in en_data:
        # Add Russian translation
        if key not in ru_data or ru_data[key]["other"].startswith("[RU]"):
            ru_data[key] = {
                "description": en_data[key]["description"],
                "other": translations["ru"]
            }
            count_ru += 1
        
        # Add Dutch translation
        if key not in nl_data or nl_data[key]["other"].startswith("[NL]"):
            nl_data[key] = {
                "description": en_data[key]["description"],
                "other": translations["nl"]
            }
            count_nl += 1

# Save updated translations
with open('internal/i18n/translations/ru.json', 'w', encoding='utf-8') as f:
    json.dump(ru_data, f, ensure_ascii=False, indent=2)

with open('internal/i18n/translations/nl.json', 'w', encoding='utf-8') as f:
    json.dump(nl_data, f, ensure_ascii=False, indent=2)

print(f"✅ Added {count_ru} REAL Russian translations")
print(f"✅ Added {count_nl} REAL Dutch translations")
print(f"Total command keys translated: {len(TRANSLATIONS)}")
