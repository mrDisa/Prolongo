from pathlib import Path

import django.db.models.deletion
from django.db import migrations, models

SQL_DIR = Path(__file__).resolve().parent.parent / "sql"


class Migration(migrations.Migration):
    """Source-system schema, verbatim from the organizers' 001_schema.up.sql."""

    initial = True

    dependencies = []

    operations = [
        migrations.RunSQL(
            sql=(SQL_DIR / "001_schema.up.sql").read_text(),
            reverse_sql=(SQL_DIR / "001_schema.down.sql").read_text(),
        ),
        migrations.CreateModel(
            name="Account",
            fields=[
                ("id", models.BigAutoField(primary_key=True, serialize=False)),
                ("email", models.TextField(unique=True)),
                ("balance", models.BigIntegerField(default=0)),
                ("withdraw_balance", models.BigIntegerField(db_column="withdrawBalance", default=0)),
                ("total_replenished", models.BigIntegerField(db_column="totalReplenished", default=0)),
                ("bonus_balance", models.BigIntegerField(db_column="bonusBalance", default=0)),
                (
                    "bonus_state",
                    models.TextField(
                        choices=[("locked", "locked"), ("unlocked", "unlocked")],
                        db_column="bonusState",
                        default="locked",
                    ),
                ),
            ],
            options={"db_table": "accounts", "managed": False},
        ),
        migrations.CreateModel(
            name="Service",
            fields=[
                ("id", models.BigAutoField(primary_key=True, serialize=False)),
                ("name", models.TextField()),
                (
                    "owner",
                    models.ForeignKey(
                        db_column="ownerId",
                        on_delete=django.db.models.deletion.PROTECT,
                        related_name="services",
                        to="source.account",
                    ),
                ),
                (
                    "payment_term",
                    models.TextField(
                        choices=[
                            ("hour", "hour"),
                            ("week", "week"),
                            ("month", "month"),
                            ("quarter_year", "quarter_year"),
                            ("half_year", "half_year"),
                            ("year", "year"),
                        ],
                        db_column="paymentTerm",
                    ),
                ),
                ("auto_prolong", models.BooleanField(db_column="autoProlong", default=False)),
                ("created_at", models.DateTimeField(db_column="createdAt")),
                ("expires_at", models.DateTimeField(db_column="expiresAt")),
                (
                    "status",
                    models.TextField(
                        choices=[
                            ("active", "active"),
                            ("blocked", "blocked"),
                            ("deleted", "deleted"),
                            ("suspended", "suspended"),
                            ("prolong_wait", "prolong_wait"),
                        ]
                    ),
                ),
                ("price", models.BigIntegerField()),
            ],
            options={"db_table": "services", "managed": False},
        ),
        migrations.CreateModel(
            name="Transaction",
            fields=[
                ("id", models.BigAutoField(primary_key=True, serialize=False)),
                ("amount", models.IntegerField()),
                (
                    "status",
                    models.TextField(
                        choices=[("not_performed", "not_performed"), ("performed", "performed")],
                        default="not_performed",
                    ),
                ),
                ("performed_at", models.DateTimeField(db_column="performedAt", null=True)),
                ("created_at", models.DateTimeField(db_column="createdAt")),
                ("type", models.TextField(choices=[("prolong", "prolong"), ("order", "order")])),
                (
                    "service",
                    models.ForeignKey(
                        db_column="serviceId",
                        on_delete=django.db.models.deletion.PROTECT,
                        related_name="transactions",
                        to="source.service",
                    ),
                ),
            ],
            options={"db_table": "transactions", "managed": False},
        ),
    ]
