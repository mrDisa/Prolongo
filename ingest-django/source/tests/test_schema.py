from pathlib import Path

import pytest
from django.conf import settings
from django.db import connection

from .conftest import organizers_file

TABLES = ("accounts", "services", "transactions")
OWN_SQL = Path(settings.BASE_DIR) / "source" / "sql" / "001_schema.up.sql"


def _without_transaction(sql):
    return "\n".join(line for line in sql.splitlines() if line.strip() not in ("BEGIN;", "COMMIT;")).strip()


def _describe(cursor, schema):
    """Columns, constraints and indexes of the source tables, with the schema name removed."""
    cursor.execute(
        """
        SELECT table_name, ordinal_position, column_name, data_type, datetime_precision,
               is_nullable, column_default, is_identity, identity_generation
        FROM information_schema.columns
        WHERE table_schema = %s AND table_name = ANY(%s)
        ORDER BY table_name, ordinal_position
        """,
        [schema, list(TABLES)],
    )
    columns = cursor.fetchall()
    cursor.execute(
        """
        SELECT rel.relname, con.conname, pg_get_constraintdef(con.oid)
        FROM pg_constraint con
        JOIN pg_class rel ON rel.oid = con.conrelid
        JOIN pg_namespace ns ON ns.oid = rel.relnamespace
        WHERE ns.nspname = %s AND rel.relname = ANY(%s)
        ORDER BY 1, 2
        """,
        [schema, list(TABLES)],
    )
    constraints = [(t, name, definition.replace(f"{schema}.", "")) for t, name, definition in cursor.fetchall()]
    cursor.execute(
        "SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = %s AND tablename = ANY(%s) ORDER BY 1, 2",
        [schema, list(TABLES)],
    )
    indexes = [(t, name, definition.replace(f"{schema}.", "")) for t, name, definition in cursor.fetchall()]
    return columns, constraints, indexes


@pytest.mark.django_db
def test_own_copy_is_organizers_sql_without_begin_commit():
    original = organizers_file("001_schema.up.sql").read_text()
    assert OWN_SQL.read_text().strip() == _without_transaction(original)


@pytest.mark.django_db
def test_migrations_create_source_tables_like_organizers_sql():
    original = _without_transaction(organizers_file("001_schema.up.sql").read_text())
    with connection.cursor() as cursor:
        cursor.execute("CREATE SCHEMA reference")
        cursor.execute(original.replace("public.", "reference."))
        public = _describe(cursor, "public")
        reference = _describe(cursor, "reference")

    columns, constraints, indexes = public
    assert len(columns) == 7 + 9 + 7
    assert columns == reference[0]
    assert constraints == reference[1]
    assert indexes == reference[2]
