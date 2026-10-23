from datetime import datetime


def set_context(cursor, *, source: str, event_time: datetime | None = None, batch_id: int | None = None):
    """Label the current transaction for the status audit trigger (sql/triggers.up.sql).

    Settings are transaction-local, so call this inside transaction.atomic().
    """
    cursor.execute("SELECT set_config('prolongo.source', %s, true)", [source])
    cursor.execute("SELECT set_config('prolongo.event_time', %s, true)", [event_time.isoformat() if event_time else ""])
    cursor.execute("SELECT set_config('prolongo.batch_id', %s, true)", [str(batch_id) if batch_id else ""])
