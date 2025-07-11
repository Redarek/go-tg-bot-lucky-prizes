# 🎰 Lucky-Prizes Telegram Bot

Телеграм-бот “Lucky-Prizes” — это мини-лотерея для пользователей с удобной
админ-панелью для менеджера.  
Бот разыгрывает случайный стикерпак **один раз для каждого пользователя**,
а дополнительные наборы можно получить после покупки на стороннем сайте.

| Стек                    | Что используется |
|-------------------------|------------------|
| **Go 1.24**             | бизнес-логика бота |
| **pgx / PostgreSQL 15** | хранение паков и попыток |
| **Docker Compose**      | локальный запуск и прод |
| **GitHub Actions** | CI / CD, миграции через `migrate` |

---

## 📸 Скриншоты

| Пользователь | Администратор |
|--------------|---------------|
| ![user](docs/img/user.png) | ![admin](docs/img/admin.png) |

---

## 🚀 Возможности

### Для пользователя
* `/start` — приветствие и кнопка **«🎲 Разыграть»**
* Проверка подписки на канал `@your_channel`
* Нативная анимация 🎰 *slot-machine*
* Одна бесплатная попытка навсегда
* Cсылка **«Получить ещё»** ведёт на сторонний сайт

### Для администратора
* `/packs` — список паков в виде кнопок  
  `[1] Cats`, `[2] DoomGuy …`
* **➕ Добавить** — диалог: название → ссылка
* **✏️ Редактировать** / **🗑️ Удалить**
* Команды доступны только владельцу `ADMIN_ID`
* Вся работа в боте через кнопки и команды

---

## 🏗️ Архитектура

cmd/main.go – точка входа, маршрутизация обновлений
pkg/
├ config/ – загрузка .env
├ db/ – подключение pgx-pool
├ models/ – структуры данных
├ repositories/ – чистый SQL CRUD + admin_states
├ services/ – бизнес-правила (одна попытка, random)
└ handlers/ – Telegram-обработчики
migrations/ – SQL-миграции (golang-migrate)
Dockerfile – multi-stage build
docker-compose.yml – bot + db
.github/workflows/ – CI / CD

---

## ⚙️ Быстрый старт

```bash
cp .env.example .env

docker-compose up --build -d

docker run --rm -v $(pwd)/migrations:/migrations \
  --network host migrate/migrate \
  -path=/migrations \
  -database "postgres://postgres:postgres@localhost:5432/stickers?sslmode=disable" up
```

В логах (docker-compose logs -f bot) появится:

```bash
Authorized as @LuckyPrizesBot
Bot started.
```

## 📦 Переменные окружения (.env)

| Переменная          | Пример                     | Описание                          |
| ------------------- | -------------------------- | --------------------------------- |
| `TELEGRAM_APITOKEN` | `123:ABC`                  | токен бота                        |
| `ADMIN_ID`          | `1130426011`               | numeric ID владельца              |
| `SHOP_URL`          | `https://shop.example.com` | ссылка «получить ещё»             |
| `SUB_CHANNEL`       | `-1001234567890`           | канал, на который надо подписаться |
| `POSTGRES_*`        | …                          | настройки БД           |

## 🧪 Добавление новой миграции

```bash
migrate create -ext sql -seq add_some_index
# появятся 00000X_add_some_index.up/down.sql
vim migrations/00000X_add_some_index.up.sql   # ваши ALTER…
```

