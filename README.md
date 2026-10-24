# Prolongo — анализатор рисков оттока

Внутренняя панель для маркетологов хостинга: показывает, **кто из клиентов может не продлить услуги, почему и сколько денег бизнес может потерять**.

Решение кейса Aéza «Анализатор рисков оттока», хакатон РНД.

## Запуск

Нужен Docker с Compose v2 и свободный порт 8080.

```bash
git clone https://github.com/mrDisa/Prolongo.git prolongo && cd prolongo
```

```bash
cp .env.example .env
```

```bash
docker compose up --build
```

Первая сборка занимает несколько минут. Когда все сервисы запустятся, открыть http://localhost:8080 и войти: `admin` / `admin123`.

При первом старте загружаются данные организаторов из `data/test-data/` (10 аккаунтов, 30 услуг, 100 транзакций). При следующих запусках данные сохраняются.

| Адрес | Что там |
| --- | --- |
| http://localhost:8080 | Панель маркетолога |
| http://localhost:8080/api/v1/docs | Swagger (после входа) |
| http://localhost:8080/admin/ | Админка данных, без второго входа (после входа в панель) |
| http://localhost:8080/metrics | Метрики Prometheus (bearer `METRICS_TOKEN` или сессия) |
| http://localhost:8080/prometheus/ | UI Prometheus (после входа) |

Без входа API, админка и UI Prometheus отвечают `401`.

### Полезные команды

| Команда | Что делает |
| --- | --- |
| `make up` | То же, что `docker compose up --build`; создаёт `.env`, если его нет |
| `make logs` | Логи всех сервисов |
| `make down` | Остановить; данные сохранятся |
| `make reset` | Остановить и удалить базу: при следующем запуске данные организаторов загрузятся заново |

Загрузить свой снимок в формате `data.json` (полная замена данных):

```bash
curl -c cookies.txt -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' http://localhost:8080/api/v1/auth/login
```

```bash
curl -b cookies.txt -X PUT -F file=@data/test-data/data.json http://localhost:8080/api/v1/source/snapshot
```

Расширенные демо-данные — ещё 300 клиентов с типичными сценариями оттока и история смен статусов за 90 дней (всё сгенерированное помечено `demo-…` и `source = demo_backfill`). Добавляются один раз, на уже запущенной базе тоже:

```bash
DEMO_DATA=1 docker compose up --build
```

Запустить симулятор потока изменений — каждые 3 секунды время источника уходит на 6 часов вперёд, услуги истекают, клиенты пополняют баланс, панель обновляется сама (`"stop"` — остановить, `"tick"` — один шаг):

```bash
curl -b cookies.txt -H 'Content-Type: application/json' -d '{"action":"start"}' http://localhost:8080/api/v1/source/simulator
```

Grafana с дашбордом «Prolongo — риск оттока» (клиенты по уровням, ожидаемые потери, MRR под риском, длительность пересчёта) — на http://localhost:3000, свой вход `admin` / `admin`, Grafana попросит сменить пароль:

```bash
docker compose --profile grafana up --build
```

### Дополнительно

| Команда | Что даст |
| --- | --- |
| `OIDC_ENABLED=1 docker compose --profile oidc up` | Вход через Keycloak (SSO): `marketer` / `marketer123`. Начать вход: http://localhost:8080/api/v1/auth/oidc/login (или кнопка «Войти через SSO»). Keycloak — http://localhost:8081, его админка — `admin` / `admin` |

## Как это устроено

```
Браузер ──► web (nginx + React) ──► core (Go): расчёт риска, API, SSE, /metrics, авторизация
                                └─► ingest (Django): снимки и события данных, история статусов, админка
                     core и ingest ──► PostgreSQL (триггеры → LISTEN/NOTIFY → пересчёт)
                     Prometheus ──► core /metrics
```

- Модель риска и денежная оценка: формулы, веса и почему такие, допущения — [`docs/MODEL.md`](docs/MODEL.md).
- Контракт API — `openapi.yml` и `docs/ОБЩЕЕ_ТЗ.md`, раздел 7.
- Как добавить новый признак риска — [`docs/ADD_FEATURE.md`](docs/ADD_FEATURE.md).

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
