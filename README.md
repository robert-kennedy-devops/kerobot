# KeroBot

KeroBot é um **cliente de usuário do Telegram** (MTProto) pronto para produção que automatiza ações de jogo no **Teletofusbot**. Ele usa `gotd/td`, PostgreSQL, logs estruturados e arquitetura modular com engine baseada em eventos.

## Recursos
- Cliente MTProto (não usa bot token)
- Detecção de botões inline com matching inteligente
- Gerenciador de estado global + engine baseada em eventos
- Fila de ações com anti-flood + retry
- Workers concorrentes (caça, combate, cura, poções, masmorra)
- Persistência em PostgreSQL + migrations
- Bot de configuração para onboarding e ajustes por usuário
- Login via QR por sessão de usuário
- Modo de captura + regras aprendidas para automação
- Endpoint de métricas
- Docker + Docker Compose

## Arquitetura
- `internal/telegram`: cliente MTProto, listener, botões, auth
- `internal/parser`: parser de mensagens e detecção de estado
- `internal/engine`: state manager, action executor, automation engine
- `internal/automation`: workers e loops
- `internal/database`: PostgreSQL e migrations
- `pkg/logger`: logger JSON com `slog`
- `pkg/retry`: helper de retry

## Estrutura do projeto
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
1. Crie o `.env`:
```
cp .env.example .env
```

2. Suba o stack e configure as credenciais do app via bot.

3. Execute:
```
docker compose up --build
```

## .env (exemplo)
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
BOT_TOKEN=seu_bot_token
ADMIN_CHAT_ID=0
```

## Bot de configuração
Comandos principais:
- `/config` mostra o menu
- `/set_api <id>` e `/set_hash <hash>` definem as credenciais do app
- `/qr` gera o QR para login de usuário
- `/capture_on` e `/capture_off` ativam o modo captura
- `/last` mostra a última captura
- `/learn_last_click <label>` cria regra com base no último botão
- `/learn_last_text <texto>` cria regra com base no último texto

## Como funciona
1. O cliente MTProto conecta e escuta o Teletofusbot.
2. Mensagens são parseadas para estados do jogo.
3. A engine decide ações.
4. Workers reforçam as ações em intervalos.
5. Ações vão para uma fila com delay, retry e anti-flood.

## Métricas
Endpoint JSON em `METRICS_ADDR`:
- `/metrics` retorna contadores de ações e mensagens.

## Observações
- Configure `API_ID` e `API_HASH` pelo bot (`/set_api` e `/set_hash`).
- O login é feito por QR (`/qr`).

## Aviso
Use com responsabilidade e respeite as regras do Telegram e do jogo.
