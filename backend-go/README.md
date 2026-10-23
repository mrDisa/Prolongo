# core — Go

Расчёт риска и денег, API для панели, авторизация, SSE, `/metrics`, Swagger.

ТЗ: [docs/ТЗ_BACKEND_GO.md](../docs/ТЗ_BACKEND_GO.md). Готово: G1 «Каркас», G2 «openapi.yml». Следующая задача — G3 «Миграции core + авторизация».

## Локальный запуск

Нужен Go 1.26 или новее. Переменные — в [ТЗ §4](../docs/ТЗ_BACKEND_GO.md); без `DATABASE_URL` сервис не стартует. В G1 к базе он ещё не подключается, поэтому подойдёт любое значение:

```bash
DATABASE_URL=postgres://prolongo:prolongo@127.0.0.1:5433/prolongo go run ./cmd/core
```

```bash
curl -i http://127.0.0.1:8000/healthz
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

## Docker

```bash
docker build -t prolongo-core .
```

В образе нет shell и `wget`, поэтому healthcheck в compose — сам бинарник: `["/core", "-healthcheck"]`. Он делает GET `/healthz` и возвращает код выхода 0 или 1.
