# KeroBot

KeroBot is a production-ready Telegram **user client** (MTProto) that automates gameplay actions for **Teletofusbot**. It uses `gotd/td`, PostgreSQL, structured logging, and a clean modular architecture with an event-based automation engine.

## Features
- MTProto user client (not a bot token)
- Inline button detection + intelligent label matching
- Global state manager + event-driven engine
- Action queue with anti-flood + retry
- Concurrent workers (hunt, combat, heal, potion, dungeon)
- PostgreSQL persistence + migrations
- Config bot for onboarding and per-user settings
- QR login per user session
- Capture mode + learned rules for automation
- Metrics endpoint for action counts and errors
- Docker + Docker Compose

## Architecture
- `internal/telegram`: MTProto client, listener, buttons, auth
- `internal/parser`: message parsing and game state detection
- `internal/engine`: state manager, action executor, automation engine
- `internal/automation`: workers and loops
- `internal/database`: PostgreSQL and migrations
- `pkg/logger`: `slog` JSON logger
- `pkg/retry`: retry helper

## Project Structure
```
kerobot/
  cmd/kerobot/main.go
  internal/
    telegram/
    parser/
    engine/
    automation/
    database/
    models/
    config/
  pkg/
    logger/
    retry/
  migrations/
  docker/
  docker-compose.yml
  .env.example
  README.md
```

## Setup
1. Create `.env`:
```
cp .env.example .env
```

2. Start the stack, then set Telegram app credentials via the config bot.

3. Run:
```
docker compose up --build
```

## .env Configuration
```
TG_PASSWORD=
TG_SESSION=./data/telegram.session
TARGET_BOT=Teletofusbot

DB_HOST=postgres
DB_PORT=5432
DB_USER=kerobot
DB_PASS=kerobot
DB_NAME=kerobot
DB_SSLMODE=disable

HUNT_INTERVAL=15s
COMBAT_INTERVAL=5s
HEAL_INTERVAL=10s
POTION_INTERVAL=30s
DUNGEON_INTERVAL=1m
HEAL_PERCENT=40
MIN_POTIONS=5

CLICK_DELAY=900ms
RATE_PER_SECOND=2
RETRY_ATTEMPTS=3
RETRY_DELAY=1500ms
METRICS_ADDR=:9090
BOT_ENABLED=true
BOT_TOKEN=your_bot_token
ADMIN_CHAT_ID=0
```

## Config Bot
Commands:
- `/config` shows the menu
- `/set_api <id>` and `/set_hash <hash>` set Telegram app credentials
- `/qr` generates a QR code for user login
- `/capture_on` and `/capture_off` toggle capture mode
- `/last` shows the last captured message and buttons
- `/learn_last_click <label>` creates a rule from the last capture
- `/learn_last_text <texto>` creates a rule from the last capture

## How It Works
1. MTProto client connects and listens to Teletofusbot updates.
2. Messages are parsed into game states.
3. Automation engine emits actions based on state.
4. Workers periodically enforce actions.
5. Actions go into a queue (anti-flood + retry + delay).

## Metrics
JSON endpoint at `METRICS_ADDR`:
- `/metrics` returns action and message counters.

## Notes
- Use the config bot to set `API_ID` and `API_HASH` with `/set_api <id>` and `/set_hash <hash>`.
- Login is done via QR in the config bot (`/qr`).

## Disclaimer
Use responsibly and respect Telegram and game rules.
