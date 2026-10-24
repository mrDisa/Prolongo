"""Point changes to the source data: the "real-time stream" of POST /api/v1/source/events."""

from django.db import IntegrityError, connection, transaction
from rest_framework import serializers

from ..exceptions import ValidationFailed
from ..models import IngestBatch, SourceClock, StatusEvent
from ..serializers import AccountDTO, ServiceDTO, TransactionDTO
from .db import set_context
from .snapshot import ACCOUNT_COLUMNS, SERVICE_COLUMNS, TRANSACTION_COLUMNS, reset_sequence, upsert_rows

ENTITIES = {
    "account": ("accounts", ACCOUNT_COLUMNS, AccountDTO),
    "service": ("services", SERVICE_COLUMNS, ServiceDTO),
    "transaction": ("transactions", TRANSACTION_COLUMNS, TransactionDTO),
}
OPS = ("upsert", "delete")


class EventsRequestDTO(serializers.Serializer):
    at = serializers.DateTimeField(required=False, allow_null=True, default=None)
    events = serializers.ListField(allow_empty=False)


class EventError(Exception):
    """An invalid event; paths are relative to events[i], e.g. "op" or "data.status"."""

    def __init__(self, path=None, message=None, errors=None):
        super().__init__(message)
        self.errors = errors if errors is not None else {path: [message]}


def _exists(cursor, table, column, value, exclude_id=None):
    sql = f'SELECT 1 FROM public.{table} WHERE "{column}" = %s'
    params = [value]
    if exclude_id is not None:
        sql += " AND id <> %s"
        params.append(exclude_id)
    cursor.execute(sql + " LIMIT 1", params)
    return cursor.fetchone() is not None


def _current_row(cursor, table, row_id):
    cursor.execute(f"SELECT row_to_json(t) FROM public.{table} t WHERE id = %s", [row_id])
    row = cursor.fetchone()
    return row[0] if row else None


def _upsert(cursor, entity, data):
    table, columns, dto_class = ENTITIES[entity]
    unknown = sorted(set(data) - set(columns))
    if unknown:
        raise EventError(f"data.{unknown[0]}", f"Неизвестное поле для {entity}")

    current = _current_row(cursor, table, data["id"]) or {}
    dto = dto_class(data={**current, **data})  # partial update: passed fields over the stored record
    if not dto.is_valid():
        raise EventError(errors={
            "data" if field == "non_field_errors" else f"data.{field}": [str(m) for m in messages]
            for field, messages in dto.errors.items()
        })
    row = dict(dto.validated_data)

    if entity == "account" and _exists(cursor, "accounts", "email", row["email"], exclude_id=row["id"]):
        raise EventError("data.email", f"Email уже занят другим аккаунтом: {row['email']}")
    if entity == "service" and not _exists(cursor, "accounts", "id", row["ownerId"]):
        raise EventError("data.ownerId", f"Нет аккаунта с id {row['ownerId']}")
    if entity == "transaction" and not _exists(cursor, "services", "id", row["serviceId"]):
        raise EventError("data.serviceId", f"Нет услуги с id {row['serviceId']}")

    upsert_rows(cursor, table, columns, [row])


def _delete(cursor, entity, row_id):
    table = ENTITIES[entity][0]
    if not _exists(cursor, table, "id", row_id):
        raise EventError("data.id", f"Нет записи {entity} с id {row_id}")
    if entity == "account":
        cursor.execute(
            'DELETE FROM public.transactions WHERE "serviceId" IN (SELECT id FROM public.services WHERE "ownerId" = %s)',
            [row_id],
        )
        cursor.execute('DELETE FROM public.services WHERE "ownerId" = %s', [row_id])
    elif entity == "service":
        cursor.execute('DELETE FROM public.transactions WHERE "serviceId" = %s', [row_id])
    cursor.execute(f"DELETE FROM public.{table} WHERE id = %s", [row_id])


def _apply_one(cursor, event):
    if not isinstance(event, dict):
        raise EventError("", "Ожидается объект с полями entity, op, data")
    entity, op, data = event.get("entity"), event.get("op"), event.get("data")
    if entity not in ENTITIES:
        raise EventError("entity", f"Недопустимое значение: {entity}")
    if op not in OPS:
        raise EventError("op", f"Недопустимое значение: {op}")
    if not isinstance(data, dict):
        raise EventError("data", "Ожидается объект")
    row_id = data.get("id")
    if not isinstance(row_id, int) or isinstance(row_id, bool) or row_id < 1:
        raise EventError("data.id", "Нужен целый id ≥ 1")
    if op == "upsert":
        _upsert(cursor, entity, data)
    else:
        _delete(cursor, entity, row_id)


def _error_path(index, path):
    return f"events[{index}].{path}" if path else f"events[{index}]"


def _advance_clock(at):
    clock = SourceClock.objects.filter(id=1).first()
    if clock is None or at > clock.data_time:
        SourceClock.objects.update_or_create(id=1, defaults={"data_time": at})


def apply_events(payload):
    request = EventsRequestDTO(data=payload)
    if not request.is_valid():
        raise ValidationFailed({k: [str(m) for m in v] for k, v in request.errors.items()})
    at = request.validated_data["at"]
    events = request.validated_data["events"]

    errors = {}
    try:
        with transaction.atomic(), connection.cursor() as cursor:
            batch = IngestBatch.objects.create(kind="event", data_time=at)
            set_context(cursor, source="event", event_time=at, batch_id=batch.id)
            if at is not None:
                _advance_clock(at)

            for index, event in enumerate(events):
                try:
                    with transaction.atomic():  # savepoint: a failed event leaves no partial writes
                        _apply_one(cursor, event)
                except EventError as exc:
                    for path, messages in exc.errors.items():
                        errors.setdefault(_error_path(index, path), []).extend(messages)
            if errors:
                raise ValidationFailed(errors, "События не применены: ошибка в данных")

            for table in ("accounts", "services", "transactions"):
                reset_sequence(cursor, table)
            counts = {"applied": len(events), "statusChanges": StatusEvent.objects.filter(batch_id=batch.id).count()}
            batch.counts = counts
            batch.save(update_fields=["counts"])
    except IntegrityError as exc:
        raise ValidationFailed({"events": [str(exc).splitlines()[0]]}, "События конфликтуют с данными в базе") from exc

    return counts
