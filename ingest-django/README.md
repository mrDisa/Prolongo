# ingest — Django + DRF

Схема БД и триггеры, загрузка снимков и событий, история статусов, симулятор, админка.

ТЗ: [docs/ТЗ_BACKEND_DJANGO.md](../docs/ТЗ_BACKEND_DJANGO.md). Первая задача — D1 «Данные».

## Локальный запуск (без compose)

```bash
python3.12 -m venv .venv && .venv/bin/pip install -r requirements.txt
```

```bash
docker run -d --name prolongo-dev-pg -e POSTGRES_DB=prolongo -e POSTGRES_USER=prolongo -e POSTGRES_PASSWORD=prolongo -p 127.0.0.1:5433:5432 postgres:16-alpine
```

```bash
DATABASE_URL=postgres://prolongo:prolongo@127.0.0.1:5433/prolongo .venv/bin/python manage.py migrate
```

Остальные переменные (`DJANGO_SECRET_KEY` и т. д.) берутся из корневого `.env`.

## Тесты

pytest-django на настоящей PostgreSQL: тесты создают отдельную базу `test_prolongo` и удаляют её после прогона, рабочие данные не трогаются.

Внутри compose (порт базы открывать не нужно):

```bash
docker compose run --rm ingest pytest
```

Локально, на базе из раздела выше (сюда же входит сверка схемы с `openapi.yml` — в контейнере она пропускается, файла там нет):

```bash
DATABASE_URL=postgres://prolongo:prolongo@127.0.0.1:5433/prolongo .venv/bin/pytest
```

| Тест | Что проверяет |
| --- | --- |
| `test_schema` | После миграций `accounts` / `services` / `transactions` совпадают с `001_schema.up.sql` организаторов: колонки, типы, ограничения, индексы |
| `test_snapshot` | data.json → 10 / 30 / 100 и часы источника; повторный PUT — 0 смен; смена статуса и удаление живой услуги → события; 422 со всеми путями; загрузка файлом |
| `test_events` | Частичный upsert меняет только переданное поле; `at` в истории и часах; ошибка откатывает весь запрос; лента и её параметры |
| `test_simulator` | Истёкшая без денег → prolong_wait, ждущая > 5 дн. → deleted, продление и бонусы; API симулятора |
| `test_auth` | Без `X-Auth-User` все `/api/v1/source/*` → 401; вход в админку по заголовку; правки из админки помечены `admin` |
| `test_triggers` | Прямой SQL пишет историю с `source = sql` и шлёт `data_changed`; на снимок — одно уведомление |
| `test_contract` | Схема API совпадает с `/api/v1/source/*` в `openapi.yml` |
