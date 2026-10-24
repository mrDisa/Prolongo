import psycopg
import pytest
from django.db import connection

from source.models import IngestBatch, SourceClock, StatusEvent
from source.services.snapshot import apply_snapshot

EMPTY = {"accounts": [], "services": [], "transactions": []}


def _listener():
    s = connection.settings_dict
    conn = psycopg.connect(host=s["HOST"], port=s["PORT"] or 5432, dbname=s["NAME"], user=s["USER"],
                           password=s["PASSWORD"], autocommit=True)
    conn.execute("LISTEN data_changed")
    return conn


@pytest.mark.django_db(transaction=True)  # NOTIFY is delivered only after a real commit
def test_direct_sql_is_audited_and_notifies(data_json):
    apply_snapshot(data_json, source="seed")
    listener = _listener()
    try:
        with connection.cursor() as cursor:
            cursor.execute("UPDATE public.services SET status = 'prolong_wait' WHERE id = 1")

        notifications = list(listener.notifies(timeout=2, stop_after=1))
        assert [n.channel for n in notifications] == ["data_changed"]
        event = StatusEvent.objects.get(service_id=1)
        assert (event.from_status, event.to_status, event.source, event.batch_id) == ("active", "prolong_wait", "sql", None)
        assert event.occurred_at == SourceClock.objects.get().data_time  # no event_time: the source clock
    finally:
        listener.close()
        # leave the database empty for the next tests
        apply_snapshot(EMPTY)
        StatusEvent.objects.all().delete()
        IngestBatch.objects.all().delete()
        SourceClock.objects.all().delete()


@pytest.mark.django_db(transaction=True)
def test_one_notification_per_snapshot(data_json):
    listener = _listener()
    try:
        apply_snapshot(data_json, source="seed")
        notifications = list(listener.notifies(timeout=2, stop_after=5))
        assert len(notifications) == 1
    finally:
        listener.close()
        apply_snapshot(EMPTY)
        StatusEvent.objects.all().delete()
        IngestBatch.objects.all().delete()
        SourceClock.objects.all().delete()
