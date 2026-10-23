# core — Go

Расчёт риска и денег, API для панели, авторизация, SSE, `/metrics`, Swagger.

ТЗ: [docs/ТЗ_BACKEND_GO.md](../docs/ТЗ_BACKEND_GO.md). Готово: G1 «Каркас». Следующая задача — G2 «openapi.yml».

## Локальный запуск

Нужен Go 1.26 или новее. Переменные — в [ТЗ §4](../docs/ТЗ_BACKEND_GO.md); без `DATABASE_URL` сервис не стартует. В G1 к базе он ещё не подключается, поэтому подойдёт любое значение:

```bash
DATABASE_URL=postgres://prolongo:prolongo@127.0.0.1:5433/prolongo go run ./cmd/core
```

```bash
curl -i http://127.0.0.1:8000/healthz
```

## Тесты

```bash
go test -race ./...
```

## Docker

```bash
docker build -t prolongo-core .
```

В образе нет shell и `wget`, поэтому healthcheck в compose — сам бинарник: `["/core", "-healthcheck"]`. Он делает GET `/healthz` и возвращает код выхода 0 или 1.
