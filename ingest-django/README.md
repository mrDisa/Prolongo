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
