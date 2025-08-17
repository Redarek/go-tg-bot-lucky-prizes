# Lucky Prizes Telegram Bot

A production-ready Telegram bot that gives each user **one random “entity”** exactly once and then sends a short follow-up CTA flow.

> **Use case (generalized):**
> The “entity” is anything with a **name** and a **text field** (e.g., URL, secondary title, promo code). Originally used for sticker packs, but you can plug in any content type that fits `name + text`.

---

## Features

* **One-time claim per user** (atomic, race-free in Postgres).
* **Random entity selection** from a configurable pool.
* **Admin flow** to add/list/edit/delete entities via bot commands.
* **Parallel, non-blocking update handling** (worker pool + rate limiter).
* **Graceful shutdown, context timeouts** for DB/API calls.
* **Dockerized** with CI/CD to GHCR and remote deploy via GitHub Actions.

---

## Tech Stack

* Go 1.24
* Telegram Bot API (`go-telegram-bot-api/v5`)
* Postgres 15 (`pgx/v5`)
* Docker / docker-compose
* GitHub Actions (build, push, deploy)

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS sticker_packs (
  id   SERIAL PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  url  TEXT NOT NULL      -- generic "text field": a URL or any text payload
);

CREATE TABLE IF NOT EXISTS user_claims (
  user_id BIGINT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS admin_states (
  user_id BIGINT PRIMARY KEY,
  state   TEXT NOT NULL,
  data    TEXT
);

CREATE TABLE bot_users (
  user_id    BIGINT PRIMARY KEY,
  created_at TIMESTAMPTZ DEFAULT now()
);
```

> You can rename `sticker_packs` to your domain (e.g., `rewards`) and keep the same columns: `name TEXT UNIQUE`, `url TEXT` (or rename `url` to `payload`).

---

## Configuration

Set via `.env` (see deploy section for auto-provision). Required keys:

| Variable            | Description                                             |
| ------------------- | ------------------------------------------------------- |
| `TELEGRAM_APITOKEN` | Telegram bot token                                      |
| `ADMIN_ID`          | Telegram user ID of the admin (int64)                   |
| `SHOP_URL`          | URL for CTA button after claim (any link)               |
| `SUB_CHANNEL_ID`    | Optional: channel ID for subscription check (`-100...`) |
| `SUB_CHANNEL_LINK`  | Public link to the channel (used in prompt)             |
| `POSTGRES_HOST`     | Postgres host (e.g., `db` in docker-compose)            |
| `POSTGRES_PORT`     | Postgres port (`5432`)                                  |
| `POSTGRES_USER`     | Postgres user                                           |
| `POSTGRES_PASSWORD` | Postgres password                                       |
| `POSTGRES_DB`       | Postgres database name                                  |

---

## Running Locally

### 1) With Docker Compose

```bash
# Start Postgres
docker compose up -d db

# Wait until Postgres is ready, then run migrations:
docker run --rm \
  --network tg-bot_default \
  -v "$(pwd)/migrations:/migrations" \
  migrate/migrate:latest \
  -path=/migrations \
  -database "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable" \
  up

# Start the bot
docker compose up -d bot
```

Ensure `.env` is present at the project root (compose picks it up).

### 2) Bare-metal (without Docker)

```bash
# 1) Start Postgres yourself and export environment variables (.env)
# 2) Run migrations using your preferred tool (or migrate CLI)
# 3) Build & run:
go mod download
go build -o bot .
./bot
```

---

## Build

### Go binary

```bash
go mod download
CGO_ENABLED=0 go build -o bot .
```

### Docker image

```bash
docker build -t ghcr.io/<owner>/<repo>:local .
```

---

## Deployment (CI/CD)

This repo includes `deploy.yml` (GitHub Actions) that:

1. Builds and pushes the image to **GHCR**.
2. SSH-es into your server, syncs `docker-compose.yml` + `migrations/`.
3. Writes `.env` on the server from GitHub Secrets.
4. Pulls the latest image and restarts the bot.
5. Runs DB migrations in a temporary container.

### Required GitHub Secrets

* `CR_PAT` — GitHub Container Registry token.
* `SSH_KEY` — private key for your deploy user.
* `SSH_USER`, `SSH_HOST` — SSH creds.
* `TELEGRAM_APITOKEN`, `ADMIN_ID`, `SHOP_URL`, `SUB_CHANNEL_ID`, `SUB_CHANNEL_LINK`.
* `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`.

> The workflow sets `POSTGRES_HOST=db` and `POSTGRES_PORT=5432` for compose.

---

## Admin Commands

* `/start` — send start screen.
* `/packs` — list all entities (rows), choose one to edit/delete.
* `/addpack` — guided flow to add new entity.
* `/draw` — force a claim+send (admin bypasses one-time restriction).

> For end-users, `/start` and `/draw` are available. Each non-admin user can claim once.

---

## Architecture Notes

* **Worker pool** for updates (parallel handling).
* **Global Telegram API rate-limiter** to avoid HTTP 429.
* **Atomic one-time claim:** `INSERT ... ON CONFLICT DO NOTHING` on `user_claims`.
* **Typed errors** (`ErrAlreadyClaimed`, `ErrNoPacks`) for clean control flow.
* **Context timeouts** around DB and Telegram operations.
* **Callback ACK** to remove loading “hourglass” in Telegram UI.

You can optionally cache the entity list in memory (periodic refresh) if `ORDER BY RANDOM()` becomes a hotspot.

---

## License

This project is licensed under the **MIT License**.
See `LICENSE` for details.
