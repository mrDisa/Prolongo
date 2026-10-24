# core — Go

Расчёт риска и денег, API для панели, авторизация, SSE, `/metrics`, Swagger.

ТЗ: [docs/ТЗ_BACKEND_GO.md](../docs/ТЗ_BACKEND_GO.md). Готово: G1 «Каркас», G2 «openapi.yml», G3 «Миграции core + авторизация», G4 «Чтение данных», G5 «Признаки + движок + деньги», G6 «Настройки». Следующая задача — G7 «Пересчёт + LISTEN».

## Локальный запуск

Нужны Go 1.26+ и PostgreSQL, в которой Django уже создал свои таблицы: при старте core ждёт `public.services` до 60 с, затем сам накатывает миграции схемы `core` и создаёт пользователя `ADMIN_USERNAME` (переменные — [ТЗ §4](../docs/ТЗ_BACKEND_GO.md)). Проще всего взять базу из [ingest-django/README.md](../ingest-django/README.md): после `manage.py migrate` она готова.

```bash
DATABASE_URL=postgres://prolongo:prolongo@127.0.0.1:5433/prolongo go run ./cmd/core
```

```bash
curl -i -c /tmp/prolongo.cookie -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' http://127.0.0.1:8000/api/v1/auth/login
```

```bash
curl -i -b /tmp/prolongo.cookie http://127.0.0.1:8000/api/v1/auth/me
```

Swagger UI — http://127.0.0.1:8000/api/v1/docs, спецификация — http://127.0.0.1:8000/api/v1/openapi.yml.

## Контракт API

Главный файл — `openapi.yml` в корне репозитория. В `openapi/openapi.yml` лежит его копия: Docker собирает core только из `backend-go/`, а `go:embed` не видит файлы выше папки модуля. После любого изменения контракта из корня репозитория:

```bash
make openapi
```

```bash
make lint-openapi
```

Если копия разошлась с корнем, упадут тест `openapi.TestCopyMatchesRoot` и проверка `contract` на PR.

## Тесты

```bash
go test -race ./...
```

Тесты базы и хранилища (`internal/db`, `internal/store`) идут на настоящей PostgreSQL и без `TEST_DATABASE_URL` пропускаются. Каждый тест создаёт рядом свою временную базу `prolongo_t_…` и удаляет её после себя (`internal/testdb`), поэтому пользователю нужно право `CREATE DATABASE`; схему и данные организаторов тест берёт из `data/test-data`. Сервер PostgreSQL для тестов:

```bash
docker run -d --name prolongo-test-pg -e POSTGRES_DB=prolongo_test -e POSTGRES_USER=prolongo -e POSTGRES_PASSWORD=prolongo -p 127.0.0.1:5434:5432 postgres:16-alpine
```

```bash
TEST_DATABASE_URL='postgres://prolongo:prolongo@127.0.0.1:5434/prolongo_test?sslmode=disable' go test -race ./...
```

В CI (`.github/workflows/backend-go.yml`) то же самое запускается на каждом PR вместе с `gofmt` и `go vet`.

## Docker

```bash
docker build -t prolongo-core .
```

В образе нет shell и `wget`, поэтому healthcheck в compose — сам бинарник: `["/core", "-healthcheck"]`. Он делает GET `/healthz` и возвращает код выхода 0 или 1.
