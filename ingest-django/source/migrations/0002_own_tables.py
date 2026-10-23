from django.db import migrations, models


class Migration(migrations.Migration):
    """Own tables next to the source schema: status history, batches, clock, simulator."""

    dependencies = [
        ("source", "0001_source_schema"),
    ]

    operations = [
        migrations.CreateModel(
            name="StatusEvent",
            fields=[
                ("id", models.BigAutoField(primary_key=True, serialize=False)),
                ("service_id", models.BigIntegerField()),
                ("account_id", models.BigIntegerField()),
                ("from_status", models.TextField(null=True)),
                ("to_status", models.TextField()),
                ("occurred_at", models.DateTimeField()),
                ("source", models.TextField()),
                ("batch_id", models.BigIntegerField(null=True)),
            ],
            options={
                "db_table": "status_events",
                "indexes": [
                    models.Index(fields=["occurred_at"], name="status_events_occurred_at_idx"),
                    models.Index(fields=["account_id"], name="status_events_account_id_idx"),
                ],
            },
        ),
        migrations.CreateModel(
            name="IngestBatch",
            fields=[
                ("id", models.BigAutoField(primary_key=True, serialize=False)),
                (
                    "kind",
                    models.TextField(
                        choices=[
                            ("seed", "seed"),
                            ("snapshot", "snapshot"),
                            ("event", "event"),
                            ("simulator", "simulator"),
                            ("demo", "demo"),
                        ]
                    ),
                ),
                ("received_at", models.DateTimeField(auto_now_add=True)),
                ("data_time", models.DateTimeField(null=True)),
                ("counts", models.JSONField(default=dict)),
            ],
            options={"db_table": "ingest_batches"},
        ),
        migrations.CreateModel(
            name="SourceClock",
            fields=[
                ("id", models.SmallIntegerField(default=1, primary_key=True, serialize=False)),
                ("data_time", models.DateTimeField()),
            ],
            options={
                "db_table": "source_clock",
                "constraints": [
                    models.CheckConstraint(condition=models.Q(("id", 1)), name="source_clock_single_row")
                ],
            },
        ),
        migrations.CreateModel(
            name="SimulatorState",
            fields=[
                ("id", models.SmallIntegerField(default=1, primary_key=True, serialize=False)),
                ("running", models.BooleanField(default=False)),
                ("interval_seconds", models.IntegerField(default=3)),
                ("tick_hours", models.FloatField(default=6)),
                ("events_generated", models.IntegerField(default=0)),
                ("seed", models.IntegerField(null=True)),
                ("updated_at", models.DateTimeField(auto_now=True)),
            ],
            options={
                "db_table": "simulator_state",
                "constraints": [
                    models.CheckConstraint(condition=models.Q(("id", 1)), name="simulator_state_single_row")
                ],
            },
        ),
    ]
