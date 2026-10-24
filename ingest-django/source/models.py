from django.core.exceptions import ValidationError
from django.core.validators import MaxValueValidator, MinValueValidator, RegexValidator
from django.db import models

# --- Source-system tables (public.accounts / services / transactions) ---
# Created verbatim from sql/001_schema.up.sql by migration 0001; Django never alters them.

BONUS_STATES = [("locked", "locked"), ("unlocked", "unlocked")]

PAYMENT_TERMS = [
    ("hour", "hour"),
    ("week", "week"),
    ("month", "month"),
    ("quarter_year", "quarter_year"),
    ("half_year", "half_year"),
    ("year", "year"),
]

SERVICE_STATUSES = [
    ("active", "active"),
    ("blocked", "blocked"),
    ("deleted", "deleted"),
    ("suspended", "suspended"),
    ("prolong_wait", "prolong_wait"),
]

TX_STATUSES = [("not_performed", "not_performed"), ("performed", "performed")]

TX_TYPES = [("prolong", "prolong"), ("order", "order")]

# Same rules as the CHECK constraints of 001_schema.up.sql, so admin forms show errors instead of a 500.
NON_NEGATIVE = [MinValueValidator(0)]
SERVICE_NAME = [RegexValidator(r"^[A-Za-z0-9-]+\Z", "Допустимы только латинские буквы, цифры и дефис")]
TX_AMOUNT = [MinValueValidator(0), MaxValueValidator(1000)]


class Account(models.Model):
    id = models.BigAutoField(primary_key=True)
    email = models.TextField(unique=True)
    balance = models.BigIntegerField(default=0, validators=NON_NEGATIVE)
    withdraw_balance = models.BigIntegerField(default=0, db_column="withdrawBalance", validators=NON_NEGATIVE)
    total_replenished = models.BigIntegerField(default=0, db_column="totalReplenished", validators=NON_NEGATIVE)
    bonus_balance = models.BigIntegerField(default=0, db_column="bonusBalance", validators=NON_NEGATIVE)
    bonus_state = models.TextField(default="locked", db_column="bonusState", choices=BONUS_STATES)

    class Meta:
        managed = False
        db_table = "accounts"

    def __str__(self):
        return self.email


class Service(models.Model):
    id = models.BigAutoField(primary_key=True)
    name = models.TextField(validators=SERVICE_NAME)
    owner = models.ForeignKey(Account, models.PROTECT, db_column="ownerId", related_name="services")
    payment_term = models.TextField(db_column="paymentTerm", choices=PAYMENT_TERMS)
    auto_prolong = models.BooleanField(default=False, db_column="autoProlong")
    created_at = models.DateTimeField(db_column="createdAt")
    expires_at = models.DateTimeField(db_column="expiresAt")
    status = models.TextField(choices=SERVICE_STATUSES)
    price = models.BigIntegerField(validators=NON_NEGATIVE)

    class Meta:
        managed = False
        db_table = "services"

    def __str__(self):
        return self.name

    def clean(self):
        if self.created_at and self.expires_at and self.expires_at < self.created_at:
            raise ValidationError({"expires_at": "Должно быть не раньше даты создания"})


class Transaction(models.Model):
    id = models.BigAutoField(primary_key=True)
    amount = models.IntegerField(validators=TX_AMOUNT)
    status = models.TextField(default="not_performed", choices=TX_STATUSES)
    performed_at = models.DateTimeField(null=True, db_column="performedAt")
    created_at = models.DateTimeField(db_column="createdAt")
    type = models.TextField(choices=TX_TYPES)
    service = models.ForeignKey(Service, models.PROTECT, db_column="serviceId", related_name="transactions")

    class Meta:
        managed = False
        db_table = "transactions"

    def __str__(self):
        return f"#{self.id} {self.type}"

    def clean(self):
        if self.status == "not_performed" and self.performed_at is not None:
            raise ValidationError({"performed_at": "У not_performed должно быть пусто"})
        if self.status == "performed":
            if self.performed_at is None:
                raise ValidationError({"performed_at": "У performed обязательно"})
            if self.created_at and self.performed_at < self.created_at:
                raise ValidationError({"performed_at": "Должно быть не раньше даты создания"})


# --- Own tables (snake_case columns, read by the Go core) ---


class StatusEvent(models.Model):
    """History of services.status changes, written by PostgreSQL triggers.

    No foreign keys on purpose: a service may disappear from the source,
    but its history must stay.
    """

    id = models.BigAutoField(primary_key=True)
    service_id = models.BigIntegerField()
    account_id = models.BigIntegerField()
    from_status = models.TextField(null=True)
    to_status = models.TextField()
    occurred_at = models.DateTimeField()
    source = models.TextField()
    batch_id = models.BigIntegerField(null=True)

    class Meta:
        db_table = "status_events"
        indexes = [
            models.Index(fields=["occurred_at"], name="status_events_occurred_at_idx"),
            models.Index(fields=["account_id"], name="status_events_account_id_idx"),
        ]

    def __str__(self):
        return f"service {self.service_id}: {self.from_status} -> {self.to_status}"


class IngestBatch(models.Model):
    KIND_CHOICES = [
        ("seed", "seed"),
        ("snapshot", "snapshot"),
        ("event", "event"),
        ("simulator", "simulator"),
        ("demo", "demo"),
    ]

    id = models.BigAutoField(primary_key=True)
    kind = models.TextField(choices=KIND_CHOICES)
    received_at = models.DateTimeField(auto_now_add=True)
    data_time = models.DateTimeField(null=True)
    counts = models.JSONField(default=dict)

    class Meta:
        db_table = "ingest_batches"

    def __str__(self):
        return f"#{self.id} {self.kind}"


class SourceClock(models.Model):
    """Single row (id = 1): the current time of the source data."""

    id = models.SmallIntegerField(primary_key=True, default=1)
    data_time = models.DateTimeField()

    class Meta:
        db_table = "source_clock"
        constraints = [models.CheckConstraint(condition=models.Q(id=1), name="source_clock_single_row")]

    def __str__(self):
        return self.data_time.isoformat()


class SimulatorState(models.Model):
    """Single row (id = 1): simulator settings and counters."""

    id = models.SmallIntegerField(primary_key=True, default=1)
    running = models.BooleanField(default=False)
    interval_seconds = models.IntegerField(default=3)
    tick_hours = models.FloatField(default=6)
    events_generated = models.IntegerField(default=0)
    seed = models.IntegerField(null=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = "simulator_state"
        constraints = [models.CheckConstraint(condition=models.Q(id=1), name="simulator_state_single_row")]

    def __str__(self):
        return "running" if self.running else "stopped"
