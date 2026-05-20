# Lucky Prizes Telegram Bot

A production-ready Telegram bot that gives each user **one random “entity”** exactly once and then sends a short follow-up CTA flow.

> **Use case (generalized):**
> The “entity” is anything with a **name** and a **text field** (e.g., URL, secondary title, promo code). Originally used for sticker packs, but you can plug in any content type that fits `name + text`.

---

## Features

* **Attempt-based claim per user** (atomic, race-free in Postgres).
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

CREATE TABLE IF NOT EXISTS user_attempts (
  user_id  BIGINT PRIMARY KEY,
  attempts INT NOT NULL DEFAULT 1 CHECK (attempts >= 0)
);

CREATE TABLE IF NOT EXISTS user_pack_claims (
  user_id    BIGINT NOT NULL,
  pack_id    INT NOT NULL REFERENCES sticker_packs(id) ON DELETE CASCADE,
  claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, pack_id)
);

CREATE TABLE IF NOT EXISTS admin_states (
  user_id BIGINT PRIMARY KEY,
  state   TEXT NOT NULL,
  data    TEXT
);

CREATE TABLE bot_users (
  user_id      BIGINT PRIMARY KEY,
  username     TEXT,
  first_name   TEXT,
  last_name    TEXT,
  created_at   TIMESTAMPTZ DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bot_message_targets (
  user_id           BIGINT PRIMARY KEY REFERENCES bot_users(user_id) ON DELETE CASCADE,
  can_message       BOOLEAN NOT NULL DEFAULT TRUE,
  last_confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error_code   INTEGER,
  last_error_text   TEXT,
  blocked_at        TIMESTAMPTZ,
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contests (
  id             SERIAL PRIMARY KEY,
  title          TEXT NOT NULL,
  status         TEXT NOT NULL CHECK (status IN ('draft','active','closed')),
  reward_pack_id INT NOT NULL REFERENCES sticker_packs(id),
  winner_user_id BIGINT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at    TIMESTAMPTZ
);

CREATE TABLE contest_channels (
  contest_id   INT NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
  channel_id   BIGINT NOT NULL,
  channel_link TEXT NOT NULL,
  position     SMALLINT NOT NULL,
  PRIMARY KEY (contest_id, channel_id),
  UNIQUE (contest_id, position)
);

CREATE TABLE contest_participants (
  contest_id INT NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
  user_id    BIGINT NOT NULL,
  joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (contest_id, user_id)
);
```

> You can rename `sticker_packs` to your domain (e.g., `rewards`) and keep the same columns: `name TEXT UNIQUE`, `url TEXT` (or rename `url` to `payload`).

---

## Configuration

Set via `.env` (see deploy section for auto-provision). Required keys:

| Variable            | Description                                             |
| ------------------- | ------------------------------------------------------- |
| `TELEGRAM_APITOKEN` | Telegram bot token                                      |
| `ADMIN_ID`          | CSV list of Telegram admin user IDs (int64), e.g. `111,222` |
| `SHOP_URL`          | URL for CTA button after claim (any link)               |
| `SUB_CHANNEL_ID`    | Optional: channel ID for subscription check (`-100...`) |
| `SUB_CHANNEL_LINK`  | Public link to the channel (used in prompt)             |
| `CONTEST_CHANNEL_1_ID` | Required for contests: channel 1 ID (`-100...`)     |
| `CONTEST_CHANNEL_1_LINK` | Public link to channel 1 (URL or `@name`)         |
| `CONTEST_CHANNEL_2_ID` | Required for contests: channel 2 ID (`-100...`)     |
| `CONTEST_CHANNEL_2_LINK` | Public link to channel 2 (URL or `@name`)         |
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
* `CONTEST_CHANNEL_1_ID`, `CONTEST_CHANNEL_1_LINK`, `CONTEST_CHANNEL_2_ID`, `CONTEST_CHANNEL_2_LINK`.
* `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`.

> The workflow sets `POSTGRES_HOST=db` and `POSTGRES_PORT=5432` for compose.

---

## Admin Commands

* `/start` — send start screen.
* `/packs` — list all entities (rows), choose one to edit/delete.
* `/addpack` — guided flow to add new entity.
* `/addattempt` — add +1 draw attempt to all users.
* `/draw` — force a claim+send (admin bypasses one-time restriction).
* `/contestadd` — create contest in `draft` (dialog: title + reward pack id).
* `/conteststart <id>` — activate contest by id.
* `/contestclose` — close active contest manually.
* `/contestpickwinner` — randomly pick winner in active contest (contest stays active).
* `/contestparticipants` — export active contest participants as XLSX.

> For end-users, `/start`, `/draw`, and `/contest` are available. `/contest` (and start button) confirms participation only after subscription to both contest channels.

---

## Architecture Notes

* **Worker pool** for updates (parallel handling).
* **Global Telegram API rate-limiter** to avoid HTTP 429.
* **Atomic attempts + history:** each draw decrements `user_attempts` and stores claimed pack in `user_pack_claims`.
* **No duplicate reward for same user:** random selection excludes packs already claimed by that user.
* **Typed errors** (`ErrNoAttempts`, `ErrNoAvailablePacks`, `ErrNoPacks`) for clean control flow.
* **Context timeouts** around DB and Telegram operations.
* **Callback ACK** to remove loading “hourglass” in Telegram UI.

You can optionally cache the entity list in memory (periodic refresh) if `ORDER BY RANDOM()` becomes a hotspot.

---

## License

This project is licensed under the **MIT License**.
See `LICENSE` for details.
