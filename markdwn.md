# Архитектура доработки: конкурсы с проверкой подписки на 2 канала

## Что добавляем

Добавляем в бота **модуль конкурсов**:
- пользователь жмёт кнопку участия;
- бот проверяет подписку на **2 обязательных канала**;
- если подписан — пользователь фиксируется как участник и получает награду (ссылка на стикерпак);
- если не подписан — бот просит подписаться и даёт кнопку повторной проверки;
- победителя можно выбрать админом: вручную после выгрузки списка или кнопкой “случайный победитель”.

---

## 1) Структура по слоям (в рамках текущего проекта)

### `pkg/config`
Добавить в конфиг:
- `CONTEST_CHANNEL_1_ID`, `CONTEST_CHANNEL_1_LINK`
- `CONTEST_CHANNEL_2_ID`, `CONTEST_CHANNEL_2_LINK`

> Не ломаем текущий `SUB_CHANNEL_ID`, он остаётся для старого flow draw.

### `pkg/models`
Новые модели:
- `Contest` (id, title, status, reward_pack_id, winner_user_id, created_at, finished_at)
- `ContestParticipant` (contest_id, user_id, joined_at)
- `ContestChannel` (contest_id, channel_id, channel_link)

### `pkg/repositories`
Новый репозиторный набор для конкурсов:
- `CreateContest(...)`
- `SetContestStatus(...)` (draft/active/closed)
- `GetActiveContest(...)`
- `AddParticipant(...)` с `ON CONFLICT DO NOTHING`
- `ListParticipants(...)`
- `SetWinner(...)`
- `PickRandomWinner(...)` (SQL `ORDER BY RANDOM() LIMIT 1` среди участников)

### `pkg/services`
Новый `ContestService`:
- `CheckRequiredSubscriptions(userID, contestID)` — проверка подписки на оба канала;
- `JoinContest(userID)` — атомарно: проверка статуса конкурса, запись участника, выдача награды;
- `PickWinner(contestID)` / `ExportParticipants(contestID)`.

### `pkg/handlers`
Расширение callback/command обработчиков:
- пользовательский callback: `contest_join`, `contest_recheck`;
- админские команды/кнопки: открыть/закрыть конкурс, выгрузить участников, выбрать победителя;
- отдельные сообщения для сценариев:
  - нет активного конкурса;
  - не подписан на 1/2 канала;
  - уже участвует;
  - участие подтверждено + ссылка на награду.

---

## 2) База данных (миграции)

Новые таблицы:

1. `contests`
- `id serial pk`
- `title text not null`
- `status text not null check (status in ('draft','active','closed'))`
- `reward_pack_id int not null references sticker_packs(id)`
- `winner_user_id bigint null`
- `created_at timestamptz default now()`
- `finished_at timestamptz null`

2. `contest_channels`
- `contest_id int not null references contests(id) on delete cascade`
- `channel_id bigint not null`
- `channel_link text not null`
- `position smallint not null`
- `primary key (contest_id, channel_id)`

3. `contest_participants`
- `contest_id int not null references contests(id) on delete cascade`
- `user_id bigint not null`
- `joined_at timestamptz default now()`
- `primary key (contest_id, user_id)`

Индекс:
- `idx_contest_participants_contest_id` на `(contest_id)` для быстрых выгрузок.

---

## 3) Логика участия (основной сценарий)

1. Пользователь нажимает **«Участвовать»** (`contest_join`).
2. Бот берёт активный конкурс.
3. Проверяет membership в двух каналах через `GetChatMember`.
4. Если подписка не полная:
   - сообщение “подпишись на оба канала”;
   - кнопки-ссылки на каналы;
   - кнопка “Проверить снова” (`contest_recheck`).
5. Если подписка ок:
   - `AddParticipant` (идемпотентно);
   - если новый участник → отправить награду (ссылку на `reward_pack_id`);
   - если уже участвует → сообщение “ты уже в списке”.

---

## 4) Админский сценарий выбора победителя

### Вариант A (ручной, базовый)
- команда: `/contest_participants`
- бот отправляет список участников (id + username, если есть)
- победителя выбираешь вручную вне бота.

### Вариант B (кнопкой в боте)
- команда/кнопка: `/contest_pickwinner`
- бот случайно выбирает участника среди `contest_participants`;
- сохраняет в `contests.winner_user_id`;
- переводит конкурс в `closed`;
- отправляет админу карточку победителя.

> Рекомендация: реализовать **оба варианта** сразу — ручной и автоматический.

---

## 5) Изменения в UX/кнопках

- В `/start` добавить блок конкурса (если есть `active`), кнопка: **«🎉 Участвовать в конкурсе»**.
- После успешного участия: короткое подтверждение + ссылка на стикерпак-награду.
- Для неподписанных: понятный call-to-action с 2 ссылками на каналы и кнопкой перепроверки.

---

## 6) Безопасность и надёжность

- Идемпотентная регистрация участия (`ON CONFLICT DO NOTHING`).
- Проверка только `member|administrator|creator`.
- Команды админа доступны только для ID из `ADMIN_ID` (CSV).
- Конкурс с `status != active` не принимает новых участников.
- Выбор победителя разрешён только если есть участники.

---

## 7) План внедрения (этапы разработки)

1. Миграции и модели конкурсов.
2. Репозиторий + сервис `ContestService`.
3. Пользовательские callback-ветки участия и проверки подписки на 2 канала.
4. Админские команды: открыть/закрыть/список участников/выбор победителя.
5. Обновление `/start` и кнопок.
6. Тестирование сценариев: подписан/не подписан/повторное участие/нет активного конкурса/пустой список при выборе победителя.

---

## Что нужно согласовать перед реализацией

1. Награда за участие: один фиксированный `reward_pack_id` на конкурс или случайный из пула?
2. Формат выгрузки участников: обычным сообщением в чат или файлом CSV.
3. При автоматическом выборе победителя: закрывать конкурс сразу или оставить активным до ручного закрытия.
