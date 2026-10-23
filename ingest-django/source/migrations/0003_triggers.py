from pathlib import Path

from django.db import migrations

SQL_DIR = Path(__file__).resolve().parent.parent / "sql"


class Migration(migrations.Migration):
    """Status history and data_changed notifications via PostgreSQL triggers."""

    dependencies = [
        ("source", "0002_own_tables"),
    ]

    operations = [
        migrations.RunSQL(
            sql=(SQL_DIR / "triggers.up.sql").read_text(),
            reverse_sql=(SQL_DIR / "triggers.down.sql").read_text(),
        ),
    ]
