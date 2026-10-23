"""Full replacement of the source data with a data.json snapshot."""

import json

from django.db import IntegrityError, connection, transaction

from ..exceptions import ValidationFailed
from ..models import IngestBatch, SourceClock, StatusEvent
from ..serializers import validate_snapshot
from .db import set_context

ACCOUNT_COLUMNS = ["id", "email", "balance", "withdrawBalance", "totalReplenished", "bonusBalance", "bonusState"]
SERVICE_COLUMNS = ["id", "name", "ownerId", "paymentTerm", "autoProlong", "createdAt", "expiresAt", "status", "price"]
TRANSACTION_COLUMNS = ["id", "amount", "status", "performedAt", "createdAt", "type", "serviceId"]


def _quoted(columns, prefix=""):
    return ", ".join(f'{prefix}"{c}"' for c in columns)


def _upsert(cursor, table, columns, rows):
    """One INSERT ... ON CONFLICT per table, like the organizers' seed.

    Rows that did not change are skipped, so the status trigger and table bloat stay quiet.
    """
    updates = ", ".join(f'"{c}" = EXCLUDED."{c}"' for c in columns if c != "id")
    cursor.execute(
        f"""
        INSERT INTO public.{table} AS t ({_quoted(columns)})
        SELECT {_quoted(columns)} FROM jsonb_populate_recordset(NULL::public.{table}, %s::jsonb)
        ON CONFLICT (id) DO UPDATE SET {updates}
        WHERE ({_quoted(columns, "t.")}) IS DISTINCT FROM ({_quoted(columns, "EXCLUDED.")})
        """,
        [json.dumps(rows, default=_json_default)],
    )


def _delete_missing(cursor, table, ids):
    cursor.execute(f"DELETE FROM public.{table} WHERE id <> ALL(%s)", [ids])
    return cursor.rowcount


def _reset_sequence(cursor, table):
    cursor.execute(
        f"""
        SELECT setval(pg_get_serial_sequence('public.{table}', 'id'),
                      COALESCE(max(id), 1), max(id) IS NOT NULL)
        FROM public.{table}
        """
    )


def _json_default(value):
    return value.isoformat()


def snapshot_data_time(data):
    """Latest moment present in the snapshot: becomes the source clock."""
    moments = [s["createdAt"] for s in data["services"]]
    moments += [t["createdAt"] for t in data["transactions"]]
    moments += [t["performedAt"] for t in data["transactions"] if t["performedAt"] is not None]
    return max(moments, default=None)


def apply_snapshot(payload, source="snapshot"):
    data, errors = validate_snapshot(payload)
    if errors:
        raise ValidationFailed(errors)

    data_time = snapshot_data_time(data)
    try:
        with transaction.atomic(), connection.cursor() as cursor:
            batch = IngestBatch.objects.create(kind=source, data_time=data_time)
            set_context(cursor, source=source, event_time=data_time, batch_id=batch.id)

            _upsert(cursor, "accounts", ACCOUNT_COLUMNS, data["accounts"])
            _upsert(cursor, "services", SERVICE_COLUMNS, data["services"])
            _upsert(cursor, "transactions", TRANSACTION_COLUMNS, data["transactions"])

            removed = {
                "transactions": _delete_missing(cursor, "transactions", [t["id"] for t in data["transactions"]]),
                "services": _delete_missing(cursor, "services", [s["id"] for s in data["services"]]),
                "accounts": _delete_missing(cursor, "accounts", [a["id"] for a in data["accounts"]]),
            }
            for table in ("accounts", "services", "transactions"):
                _reset_sequence(cursor, table)

            if data_time is not None:
                SourceClock.objects.update_or_create(id=1, defaults={"data_time": data_time})

            counts = {
                "accounts": len(data["accounts"]),
                "services": len(data["services"]),
                "transactions": len(data["transactions"]),
                "statusChanges": StatusEvent.objects.filter(batch_id=batch.id).count(),
                "removed": {k: removed[k] for k in ("accounts", "services", "transactions")},
            }
            batch.counts = counts
            batch.save(update_fields=["counts"])
    except IntegrityError as exc:
        # Validation covers the snapshot itself; this is a clash with rows already in the database,
        # e.g. two accounts swapping emails.
        if "accounts_email_key" in str(exc):
            details = {"accounts": ["Email уже занят другим аккаунтом в базе (перестановка email между id)"]}
        else:
            details = {"body": [str(exc).splitlines()[0]]}
        raise ValidationFailed(details, "Снимок конфликтует с данными в базе") from exc

    return {"batchId": batch.id, **counts}
