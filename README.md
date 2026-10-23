# Prolongo — анализатор рисков оттока

Внутренняя панель для маркетологов хостинга: показывает, **кто из клиентов может не продлить услуги, почему и сколько денег бизнес может потерять**.

Решение кейса Aéza «Анализатор рисков оттока», хакатон РНД.

> Статус: каркас репозитория. Сервисы появляются по задачам из `docs/` — раздел «Запуск» заработает после задач D5 / G1 / F1.

## Запуск

Нужен Docker с Compose v2.

```bash
cp .env.example .env
```

```bash
docker compose up --build
```

Открыть http://localhost:8080 и войти: `admin` / `admin123`.

| Адрес | Что там |
| --- | --- |
| http://localhost:8080 | Панель маркетолога |
| http://localhost:8080/api/v1/docs | Swagger (после входа) |
| http://localhost:8080/admin/ | Админка данных (после входа) |
| http://localhost:8080/metrics | Метрики Prometheus (bearer `METRICS_TOKEN`) |
| http://localhost:8080/prometheus/ | UI Prometheus (после входа) |

Дополнительно:

```bash
DEMO_DATA=1 docker compose up --build
```

```bash
docker compose --profile grafana up
```

```bash
OIDC_ENABLED=1 docker compose --profile oidc up
```

При первом старте загружаются данные организаторов из `data/test-data/` (10 аккаунтов, 30 услуг, 100 транзакций).

## Как это устроено

```
Браузер ──► web (nginx + React) ──► core (Go): расчёт риска, API, SSE, /metrics, авторизация
                                └─► ingest (Django): снимки и события данных, история статусов, админка
                     core и ingest ──► PostgreSQL (триггеры → LISTEN/NOTIFY → пересчёт)
                     Prometheus ──► core /metrics
```

- Модель риска и денежная оценка — `docs/ОБЩЕЕ_ТЗ.md`, разделы 4–5.
- Контракт API — `openapi.yml` и `docs/ОБЩЕЕ_ТЗ.md`, раздел 7.
- Как добавить новый признак риска — `docs/ТЗ_BACKEND_GO.md`, раздел 19 (после G13 — `docs/ADD_FEATURE.md`).

## Структура

```
openapi.yml        контракт API — источник правды
backend-go/        core: Go
ingest-django/     ingest: Django + DRF
frontend/          web: React + TypeScript + Vite
deploy/            nginx, Prometheus, Grafana, Keycloak
data/test-data/    данные организаторов без изменений
docs/              ТЗ команды
```

## Для команды

Правила работы с ветками и pull request — [CONTRIBUTING.md](CONTRIBUTING.md). После клонирования один раз:

```bash
make setup
```
