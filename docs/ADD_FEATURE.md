# Как добавить признак риска

Признак — это один файл на Go примерно на 30 строк. Движок, API настроек, панель «Модель риска», предпросмотр, карточка клиента и `/metrics` подхватывают его сами: **фронт, база и контракт API не меняются.**

Ниже — полный пример: признак «Пустой баланс» (`low_balance`). Именно так он проверялся при написании этой инструкции: после шагов 1–4 он появился в `GET /api/v1/model/settings`, в панели ползунком с весом 5 и в метрике `prolongo_model_feature_weight{feature="low_balance"}`.

## Шаг 1. Создать файл признака

`backend-go/internal/features/low_balance.go`:

```go
package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/textfmt"
)

// init запускается до main: признак сам записывается в реестр.
func init() { Register(lowBalance{}) }

// lowBalance: на балансе меньше €1 — клиент не сможет оплатить даже мелочь.
type lowBalance struct{}

func (lowBalance) Meta() Meta {
	return Meta{
		Key:           "low_balance",            // уникальный, snake_case; попадает в API и метрики
		Title:         "Пустой баланс",          // название в панели
		Description:   "На балансе меньше €1.",  // 1–2 предложения под названием
		Scope:         ScopeAccount,             // ScopeAccount — раз на клиента, ScopeService — на каждую услугу
		DefaultWeight: 5,                        // баллы при f = 1; к нему возвращает «Сбросить»
		Min:           0,                        // границы ползунка;
		Max:           20,                       // DefaultWeight должен быть внутри
		Action:        "Напомнить о пополнении", // рекомендация маркетологу; пусто — без рекомендации
	}
}

// Compute возвращает значение 0..1 и текст причины для карточки.
// 0 значит «не сработал» — тогда и текст не показывается.
// Для ScopeAccount второй аргумент (услуга) всегда nil.
func (lowBalance) Compute(c *Context, _ *domain.Service) (float64, string) {
	if c.Account.Balance >= 100 { // суммы — в центах
		return 0, ""
	}
	return 1, fmt.Sprintf("На балансе %s", textfmt.EUR(c.Account.Balance))
}
```

Что доступно в `Context` (`internal/features/feature.go`):

| Поле | Что это |
| --- | --- |
| `Now` | «Сейчас» модели (по настройке часов) |
| `HorizonDays` | Горизонт прогноза, дней |
| `Account` | Клиент: `Balance`, `BonusBalance`, `BonusUnlocked`, `TotalReplenished`… (центы) |
| `Services` / `Live` | Все услуги клиента / только живые |
| `Tx` | Все транзакции его услуг |
| `NeedH`, `Funds`, `MRR` | Сколько надо заплатить за горизонт, сколько есть денег, месячная выручка |

Помощники рядом, в `helpers.go`: `Clamp`, `DaysLeft`, `ChargesH`, `MRR`. Деньги в тексте — `textfmt.EUR`, дни — `textfmt.Days`.

## Шаг 2. Написать тест

Тесты признаков лежат в `backend-go/internal/features/features_test.go`; добавьте рядом свой (или отдельный `low_balance_test.go` в том же пакете). Проверьте ноль, срабатывание, граничное значение и текст:

```go
func TestLowBalance(t *testing.T) {
	cases := []struct {
		name    string
		balance int64
		value   float64
		reason  string
	}{
		{"enough money", 5000, 0, ""},
		{"exactly €1: not low", 100, 0, ""},
		{"99 cents", 99, 1, "На балансе €0,99"},
		{"empty", 0, 1, "На балансе €0,00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Context{Account: domain.Account{Balance: tc.balance}}
			v, r := compute(t, "low_balance", c, nil)
			assert.InDelta(t, tc.value, v, 1e-9)
			assert.Equal(t, tc.reason, r)
		})
	}
}
```

Для признака уровня услуги удобен готовый помощник: `runServiceCases(t, "<key>", []serviceCase{...})` и `svc(status, term, daysLeft, autoProlong)` — см. `TestProlongWait`.

## Шаг 3. Добавить ключ в `TestRegistry`

`TestRegistry` в том же файле намеренно перечисляет все признаки списком — чтобы набор признаков менялся только осознанно. Впишите новый ключ на его место по алфавиту:

```go
assert.Equal(t, []string{"churn_history", "expiring_no_autoprolong", "failed_payments", "insufficient_funds",
	"low_balance", "loyal_payer", "new_client", "prolong_wait", "short_billing", "suspended"}, keys, ...)
```

Этот же тест проверяет каждый признак: есть название и описание, правильный уровень, вес по умолчанию внутри [Min, Max], повторный ключ вызывает панику. Остальные тесты число признаков берут из реестра — их трогать не нужно.

## Шаг 4. Запустить тесты и пересобрать core

```bash
cd backend-go && go test ./...
docker compose up --build core
```

Если признак срабатывает на данных организаторов (`data/test-data/data.json`), упадёт эталонный тест движка `internal/engine/engine_test.go`: он закрепляет баллы и деньги клиентов. Это ожидаемо — посмотрите, как изменились числа, и, если они осмысленны, обновите эталон в тесте и в `docs/MODEL.md` §10.

## Шаг 5. Проверить, что признак появился

- **API:** `GET /api/v1/model/settings` — новый элемент в `features` с весом по умолчанию.
- **Панель:** «Модель риска» — карточка «Пустой баланс» с ползунком и подписью «Сейчас срабатывает у N клиентов».
- **Карточка клиента:** вклад и текст причины, если признак сработал; рекомендация — если вклад ≥ 5 баллов.
- **Метрики:** `prolongo_model_feature_weight{feature="low_balance"} 5`.

Сохранённые настройки менять не нужно: при старте core сливает их с реестром, и новый признак получает `DefaultWeight` (`internal/settings`). Признак, удалённый из кода, из сохранённых настроек не стирается — просто перестаёт учитываться.

## Если признаку нужны новые данные

Например, история обращений в поддержку. Остальной код по-прежнему не меняется, добавляется только путь данных:

1. **`internal/domain/domain.go`** — поле или новый тип (например, `Ticket`) и место в `Dataset`.
2. **`internal/store/source.go`** — запрос к таблице источника; все запросы идут в одной транзакции REPEATABLE READ, поэтому данные согласованы.
3. **`internal/domain/json.go`** — если поле должно читаться и из `data.json` в тестах.
4. **`internal/features/feature.go`** — поле в `Context`, и **`helpers.go` → `NewContext`** — заполнить его для клиента.
5. Новая таблица источника — это зона ingest (Django): схема, триггеры `NOTIFY data_changed` для пересчёта.

## Чек-лист

- [ ] Ключ уникальный, в snake_case.
- [ ] `Compute` возвращает значение от 0 до 1; при 0 — пустой текст.
- [ ] Текст причины и действие — по-русски, конкретно, с цифрами клиента.
- [ ] Вес по умолчанию внутри [Min, Max]; у «защитного» признака вес отрицательный (как у `loyal_payer`: −50…0).
- [ ] Тест признака и ключ в `TestRegistry`.
- [ ] `go test ./...` зелёный; эталон обновлён, если признак меняет числа на данных организаторов.
- [ ] Строка в таблице признаков `docs/MODEL.md` §4 с формулой и объяснением веса.
