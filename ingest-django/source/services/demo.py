"""Extended demo data: realistic churn scenarios on top of the organizers' dataset.

Everything generated here is marked: emails demo-NNNN@example.test, history source=demo_backfill.
"""

import calendar
import math
import random
from dataclasses import dataclass, field
from datetime import datetime, timedelta

from django.db import connection, transaction

from ..models import Account, IngestBatch, Service, StatusEvent, Transaction
from .db import set_context
from .simulator import TERM_MONTHS, add_term, current_data_time
from .snapshot import reset_sequence

EMAIL_PREFIX = "demo-"
GRACE = timedelta(days=5)
TERM_DAYS = {"hour": 1 / 24, "week": 7, "month": 30, "quarter_year": 91, "half_year": 182, "year": 365}

# Shares from ТЗ_BACKEND_DJANGO §11; the remainder of the rounding goes to "healthy".
SCENARIOS = [
    ("healthy", 0.30),
    ("expiring", 0.15),
    ("short_money", 0.15),
    ("waiting", 0.10),
    ("suspended", 0.10),
    ("churned", 0.10),
    ("new", 0.10),
]


def shift(value, term, periods):
    """Move a date by a whole number of billing periods (negative = back)."""
    if term == "hour":
        return value + timedelta(hours=periods)
    if term == "week":
        return value + timedelta(weeks=periods)
    month_index = value.year * 12 + value.month - 1 + TERM_MONTHS[term] * periods
    year, month = divmod(month_index, 12)
    month += 1
    day = min(value.day, calendar.monthrange(year, month)[1])
    return value.replace(year=year, month=month, day=day)


def price_for(term, monthly):
    """Per-period price from a monthly VPS price in cents."""
    return {
        "hour": max(1, monthly // 720),
        "week": monthly * 7 // 30,
        "month": monthly,
        "quarter_year": monthly * 3,
        "half_year": monthly * 6,
        "year": monthly * 12,
    }[term]


def charges_30d(term):
    return math.ceil(30 / TERM_DAYS[term])


@dataclass
class DemoService:
    term: str
    price: int
    status: str
    auto_prolong: bool
    created_at: datetime
    expires_at: datetime
    payments: list = field(default_factory=list)  # (type, moment, performed)


@dataclass
class DemoAccount:
    scenario: str
    services: list = field(default_factory=list)


class Generator:
    def __init__(self, now, rng):
        self.now = now
        self.rng = rng

    def monthly(self):
        return self.rng.randint(300, 4000)  # VPS for 3..40 EUR a month

    def paid_until(self, created, term, end):
        """Order at creation and a performed prolong at every period boundary up to end."""
        payments = [("order", created, True)]
        expires = add_term(created, term)
        while expires <= end:
            payments.append(("prolong", expires, True))
            expires = add_term(expires, term)
        return payments, expires

    def live(self, term, status, auto, periods_back, jitter_hours=0):
        created = shift(self.now, term, -periods_back) - timedelta(hours=jitter_hours)
        payments, expires = self.paid_until(created, term, self.now)
        return DemoService(term, price_for(term, self.monthly()), status, auto, created, expires, payments)

    def paid_up_to(self, expires, term, periods):
        """Paid history ending exactly at expires: order, then a prolong at every earlier boundary."""
        boundaries = [shift(expires, term, -i) for i in range(periods, 0, -1)]
        payments = [("order", boundaries[0], True)] + [("prolong", b, True) for b in boundaries[1:]]
        return boundaries[0], payments

    def expiring_soon(self, term, auto, days_left):
        expires = self.now + timedelta(days=days_left)
        created, payments = self.paid_up_to(expires, term, self.rng.randint(1, 6))
        return DemoService(term, price_for(term, self.monthly()), "active", auto, created, expires, payments)

    def lapsed(self, term, status, expired_at):
        """Paid until expired_at, then a failed prolong at that moment."""
        periods = self.rng.randint(1, 6) if term != "hour" else self.rng.randint(2, 24)
        created, payments = self.paid_up_to(expired_at, term, periods)
        payments.append(("prolong", expired_at, False))
        return DemoService(term, price_for(term, self.monthly()), status, self.rng.random() < 0.5, created, expired_at, payments)

    def extra(self):
        """A background service for multi-service accounts: paid and auto-renewed."""
        term = self.rng.choice(["month", "month", "month", "quarter_year", "year", "week"])
        return self.live(term, "active", True, self.rng.randint(1, 4), self.rng.randint(0, 72))

    # --- scenarios ---

    def build(self, scenario):
        rng = self.rng
        account = DemoAccount(scenario)
        extra_count = rng.choice([0, 0, 1, 1, 2])
        if scenario == "healthy":
            term = rng.choice(["month", "month", "month", "year"])
            account.services.append(self.live(term, "active", True, rng.randint(3, 10) if term == "month" else 3,
                                              rng.randint(0, 20 * 24)))
        elif scenario == "expiring":
            term = rng.choice(["month", "month", "quarter_year", "week"])
            days = rng.uniform(1, 10) if term != "week" else rng.uniform(1, 6)
            account.services.append(self.expiring_soon(term, False, days))
        elif scenario == "short_money":
            term = rng.choice(["month", "week", "hour"])
            if term == "hour":
                service = self.live("hour", "active", True, rng.randint(5, 48))
            else:
                service = self.expiring_soon(term, True, rng.uniform(0.5, 14 if term == "month" else 6))
            account.services.append(service)
        elif scenario == "waiting":
            term = rng.choice(["month", "month", "week", "hour"])
            account.services.append(self.lapsed(term, "prolong_wait", self.now - timedelta(days=rng.uniform(0, 4))))
        elif scenario == "suspended":
            term = rng.choice(["month", "month", "week", "quarter_year"])
            account.services.append(self.live(term, "suspended", rng.random() < 0.5, rng.randint(1, 6), rng.randint(0, 72)))
        elif scenario == "churned":
            for _ in range(rng.randint(1, 2)):
                gone = self.now - GRACE - timedelta(days=rng.uniform(0, 80))
                account.services.append(self.lapsed(rng.choice(["month", "week", "hour"]), "deleted", gone))
            account.services.append(self.expiring_soon("month", rng.random() < 0.5, rng.uniform(1, 25)))
            extra_count = 0
        elif scenario == "new":
            term = rng.choice(["month", "month", "week"])
            created = self.now - timedelta(days=rng.uniform(0, 20 if term == "month" else 5))
            payments, expires = self.paid_until(created, term, self.now)
            account.services.append(DemoService(term, price_for(term, self.monthly()), "active", rng.random() < 0.5,
                                                created, expires, payments))
            extra_count = 0
        for _ in range(extra_count):
            account.services.append(self.extra())
        return account

    def money(self, account):
        """Balance, replenishments and bonuses consistent with the scenario and paid history."""
        rng = self.rng
        live = [s for s in account.services if s.status in ("active", "suspended", "prolong_wait")]
        need = sum(s.price * charges_30d(s.term) for s in live)
        spent = sum(s.price for s in account.services for kind, _, ok in s.payments if ok)
        balance = {
            "healthy": need * rng.randint(3, 8),
            "expiring": int(need * rng.uniform(1, 3)),
            "suspended": int(need * rng.uniform(1, 3)),
            "short_money": rng.randint(0, max(0, need // 2)),
            "waiting": rng.randint(0, max(0, min(s.price for s in live) - 1)),
            "churned": rng.randint(0, need),
            "new": rng.randint(0, max(0, spent // 5)),
        }[account.scenario]
        locked = account.scenario in ("short_money", "waiting")
        return {
            "balance": balance,
            "withdraw_balance": rng.randint(0, balance // 5),
            "total_replenished": spent + balance,
            "bonus_balance": rng.randint(0, 2000),
            "bonus_state": "locked" if locked or rng.random() < 0.7 else "unlocked",
        }


def _scenario_plan(total, rng):
    plan = []
    for name, share in SCENARIOS:
        plan += [name] * int(total * share)
    plan += ["healthy"] * (total - len(plan))
    rng.shuffle(plan)
    return plan


def _backfill(service, owner_id, now, since, batch_id, rng):
    """History before the system existed: transitions to prolong_wait and deleted."""
    events = []
    went_waiting = service.expires_at
    if service.status == "deleted":
        deleted_at = service.expires_at + GRACE
        if since <= deleted_at <= now:
            events.append(("active", "prolong_wait", went_waiting))
            events.append(("prolong_wait", "deleted", deleted_at))
    elif service.status == "prolong_wait" and rng.random() < 0.5:
        events.append(("active", "prolong_wait", went_waiting))
    return [
        StatusEvent(service_id=service.id, account_id=owner_id, from_status=a, to_status=b,
                    occurred_at=at, source="demo_backfill", batch_id=batch_id)
        for a, b, at in events
    ]


def generate_demo(accounts=300, seed=7, backfill_days=90):
    if Account.objects.filter(email__startswith=EMAIL_PREFIX).exists():
        return {"skipped": True}

    rng = random.Random(seed)
    now = current_data_time()
    generator = Generator(now, rng)
    plan = _scenario_plan(accounts, rng)

    with transaction.atomic():
        batch = IngestBatch.objects.create(kind="demo", data_time=now)
        with connection.cursor() as cursor:
            set_context(cursor, source="demo", batch_id=batch.id)
            for table in ("accounts", "services", "transactions"):
                reset_sequence(cursor, table)

        first_id = (Account.objects.order_by("-id").values_list("id", flat=True).first() or 0) + 1
        built = [generator.build(scenario) for scenario in plan]
        account_rows = [
            Account(id=first_id + n, email=f"{EMAIL_PREFIX}{n + 1:04d}@example.test", **generator.money(a))
            for n, a in enumerate(built)
        ]
        Account.objects.bulk_create(account_rows, batch_size=500)

        service_rows, service_specs = [], []
        for n, (row, demo) in enumerate(zip(account_rows, built)):
            for k, spec in enumerate(demo.services, start=1):
                service_rows.append(Service(
                    name=f"demo-{n + 1:04d}-{k}", owner_id=row.id, payment_term=spec.term,
                    auto_prolong=spec.auto_prolong, created_at=spec.created_at, expires_at=spec.expires_at,
                    status=spec.status, price=spec.price,
                ))
                service_specs.append(spec)
        Service.objects.bulk_create(service_rows, batch_size=1000)

        tx_rows, history = [], []
        since = now - timedelta(days=backfill_days)
        for service, spec in zip(service_rows, service_specs):
            for kind, moment, performed in spec.payments:
                # small jitter like real payments, never later than the source clock
                created = min(moment + timedelta(seconds=rng.randint(0, 300)), now)
                tx_rows.append(Transaction(
                    service_id=service.id, type=kind, amount=min(spec.price, 1000),
                    status="performed" if performed else "not_performed", created_at=created,
                    performed_at=min(created + timedelta(seconds=rng.randint(1, 60)), now) if performed else None,
                ))
            history += _backfill(service, service.owner_id, now, since, batch.id, rng)
        Transaction.objects.bulk_create(tx_rows, batch_size=2000)
        StatusEvent.objects.bulk_create(history, batch_size=2000)

        with connection.cursor() as cursor:
            for table in ("accounts", "services", "transactions"):
                reset_sequence(cursor, table)

        counts = {
            "accounts": len(account_rows),
            "services": len(service_rows),
            "transactions": len(tx_rows),
            "backfillEvents": len(history),
            "scenarios": {name: plan.count(name) for name, _ in SCENARIOS},
        }
        batch.counts = counts
        batch.save(update_fields=["counts"])
    return counts
