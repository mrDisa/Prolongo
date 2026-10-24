"""Simulated flow of source changes: one tick moves the source clock and lets services live.

Each tick runs in one transaction labelled source=simulator, so the triggers write the
status history and send a single data_changed notification.
"""

import calendar
import random
from datetime import timedelta

from django.db import connection, transaction
from django.utils import timezone

from ..models import Account, IngestBatch, Service, SimulatorState, SourceClock, StatusEvent, Transaction
from .db import set_context

GRACE = timedelta(days=5)  # prolong_wait -> deleted
MAX_PROLONGS = 200  # per service and tick, bounds hourly services far in the past
LIVE_PAYING = ("active", "suspended")
TERM_MONTHS = {"month": 1, "quarter_year": 3, "half_year": 6, "year": 12}


def add_term(value, term):
    """Same period arithmetic as the organizers' generate.py (calendar months)."""
    if term == "hour":
        return value + timedelta(hours=1)
    if term == "week":
        return value + timedelta(weeks=1)
    month_index = value.year * 12 + value.month - 1 + TERM_MONTHS[term]
    year, month = divmod(month_index, 12)
    month += 1
    day = min(value.day, calendar.monthrange(year, month)[1])
    return value.replace(year=year, month=month, day=day)


def get_state():
    state, _ = SimulatorState.objects.get_or_create(id=1)
    return state


def current_data_time():
    clock = SourceClock.objects.filter(id=1).first()
    return clock.data_time if clock else timezone.now()


class Wallet:
    """Account money during a tick: bonuses first (if unlocked), then the balance."""

    def __init__(self, account):
        self.account = account
        self.changed = False

    def available(self):
        bonus = self.account.bonus_balance if self.account.bonus_state == "unlocked" else 0
        return bonus + self.account.balance

    def pay(self, price):
        if self.available() < price:
            return False
        a = self.account
        from_bonus = min(price, a.bonus_balance) if a.bonus_state == "unlocked" else 0
        a.bonus_balance -= from_bonus
        a.balance -= price - from_bonus
        a.withdraw_balance = min(a.withdraw_balance, a.balance)  # withdrawable money is part of the balance
        self.changed = True
        return True

    def top_up(self, amount):
        self.account.balance += amount
        self.account.total_replenished += amount
        self.changed = True


class Tick:
    def __init__(self, data_time, rng):
        self.now = data_time
        self.rng = rng
        self.wallets = {}
        self.random_count = 0

    def wallet(self, account_id):
        if account_id not in self.wallets:
            self.wallets[account_id] = Wallet(Account.objects.select_for_update().get(id=account_id))
        return self.wallets[account_id]

    def add_transaction(self, service, kind, performed):
        Transaction.objects.create(
            service=service,
            type=kind,
            amount=min(service.price, 1000),  # schema allows 0..1000
            status="performed" if performed else "not_performed",
            created_at=self.now,
            performed_at=self.now if performed else None,
        )

    def renew_expired(self):
        expired = Service.objects.select_for_update().filter(status__in=LIVE_PAYING, expires_at__lte=self.now)
        for service in expired.order_by("id"):
            if service.auto_prolong:
                wallet = self.wallet(service.owner_id)
                for _ in range(MAX_PROLONGS):
                    if service.expires_at > self.now or not wallet.pay(service.price):
                        break
                    service.expires_at = add_term(service.expires_at, service.payment_term)
                    self.add_transaction(service, "prolong", performed=True)
                if service.expires_at > self.now:
                    service.save(update_fields=["expires_at"])
                    continue
            service.status = "prolong_wait"
            service.save(update_fields=["status", "expires_at"])
            self.add_transaction(service, "prolong", performed=False)

    def delete_overdue(self):
        overdue = Service.objects.select_for_update().filter(status="prolong_wait", expires_at__lte=self.now - GRACE)
        for service in overdue.order_by("id"):
            service.status = "deleted"
            service.save(update_fields=["status"])

    # --- random customer behaviour ---

    def top_up_random_account(self):
        account_id = self.rng.choice(list(Account.objects.order_by("id").values_list("id", flat=True)) or [None])
        if account_id is None:
            return False
        wallet = self.wallet(account_id)
        wallet.top_up(self.rng.randint(10, 200) * 100)
        waiting = Service.objects.select_for_update().filter(owner_id=account_id, status="prolong_wait").order_by("id")
        for service in waiting:
            if not wallet.pay(service.price):
                break
            service.status = "active"
            service.expires_at = add_term(self.now, service.payment_term)
            service.save(update_fields=["status", "expires_at"])
            self.add_transaction(service, "prolong", performed=True)
        return True

    def _random_service(self, **filters):
        ids = list(Service.objects.filter(**filters).order_by("id").values_list("id", flat=True))
        return Service.objects.select_for_update().get(id=self.rng.choice(ids)) if ids else None

    def disable_auto_prolong(self):
        service = self._random_service(status="active", auto_prolong=True)
        if service is None:
            return False
        service.auto_prolong = False
        service.save(update_fields=["auto_prolong"])
        return True

    def suspend_random_service(self):
        service = self._random_service(status="active")
        if service is None:
            return False
        service.status = "suspended"
        service.save(update_fields=["status"])
        return True

    def new_order(self):
        term = self.rng.choice(["hour", "week", "month", "month", "quarter_year", "half_year", "year"])
        monthly = self.rng.randint(300, 4000)  # VPS for 3..40 EUR a month
        price = {
            "hour": max(1, monthly // 720),
            "week": monthly * 7 // 30,
            "month": monthly,
            "quarter_year": monthly * 3,
            "half_year": monthly * 6,
            "year": monthly * 12,
        }[term]
        account_ids = Account.objects.order_by("id").values_list("id", flat=True)
        candidates = [a for a in account_ids if self.wallet(a).available() >= price]
        if not candidates:
            return False
        account_id = self.rng.choice(candidates)
        self.wallet(account_id).pay(price)
        service = Service.objects.create(
            name="sim-pending",
            owner_id=account_id,
            payment_term=term,
            auto_prolong=self.rng.random() < 0.6,
            created_at=self.now,
            expires_at=add_term(self.now, term),
            status="active",
            price=price,
        )
        service.name = f"service-{service.id:04d}"
        service.save(update_fields=["name"])
        self.add_transaction(service, "order", performed=True)
        return True

    def random_events(self):
        actions = [self.top_up_random_account, self.disable_auto_prolong, self.suspend_random_service, self.new_order]
        for _ in range(self.rng.randint(1, 3)):
            if self.rng.choice(actions)():
                self.random_count += 1

    def save_wallets(self):
        for wallet in self.wallets.values():
            if wallet.changed:
                wallet.account.save(update_fields=["balance", "withdraw_balance", "total_replenished", "bonus_balance"])


def run_tick():
    """One simulation step. Returns counts of what happened."""
    with transaction.atomic():
        state = SimulatorState.objects.select_for_update().get_or_create(id=1)[0]
        data_time = current_data_time() + timedelta(hours=state.tick_hours)
        seed = f"{state.seed}:{data_time.isoformat()}" if state.seed is not None else None
        tick = Tick(data_time, random.Random(seed))

        batch = IngestBatch.objects.create(kind="simulator", data_time=data_time)
        with connection.cursor() as cursor:
            set_context(cursor, source="simulator", event_time=data_time, batch_id=batch.id)
        SourceClock.objects.update_or_create(id=1, defaults={"data_time": data_time})

        tick.renew_expired()
        tick.delete_overdue()
        tick.random_events()
        tick.save_wallets()

        status_changes = StatusEvent.objects.filter(batch_id=batch.id).count()
        counts = {"statusChanges": status_changes, "randomEvents": tick.random_count}
        batch.counts = counts
        batch.save(update_fields=["counts"])

        state.events_generated += status_changes + tick.random_count
        state.save(update_fields=["events_generated", "updated_at"])
    return {"dataTime": data_time, **counts}
