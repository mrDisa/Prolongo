import pytest

from source.models import Account, SourceClock, StatusEvent

from .conftest import post_json

URL = "/api/v1/source/events"


@pytest.mark.django_db
def test_events_partial(api, loaded):
    before = Account.objects.values().get(id=5)

    response = post_json(api, URL, {"events": [{"entity": "account", "op": "upsert", "data": {"id": 5, "balance": 0}}]})

    assert response.status_code == 200
    assert response.json() == {"applied": 1, "statusChanges": 0}
    after = Account.objects.values().get(id=5)
    assert {k for k in before if before[k] != after[k]} == {"balance"}
    assert after["balance"] == 0


@pytest.mark.django_db
def test_events_status_change_uses_at(api, loaded):
    body = {
        "at": "2026-10-19T15:05:00.000Z",
        "events": [{"entity": "service", "op": "upsert", "data": {"id": 25, "status": "active", "expiresAt": "2026-10-19T16:05:00.000Z"}}],
    }

    response = post_json(api, URL, body)

    assert response.json()["statusChanges"] == 1
    event = StatusEvent.objects.get(service_id=25)
    assert (event.from_status, event.to_status, event.source) == ("prolong_wait", "active", "event")
    assert event.occurred_at.isoformat() == "2026-10-19T15:05:00+00:00"
    assert SourceClock.objects.get().data_time == event.occurred_at


@pytest.mark.django_db
def test_events_error_rolls_back_whole_request(api, loaded):
    body = {"events": [
        {"entity": "account", "op": "upsert", "data": {"id": 1, "balance": 1}},
        {"entity": "account", "op": "upsert", "data": {"id": 50, "email": "new@example.test"}},
        {"entity": "service", "op": "upsert", "data": {"id": 5, "status": "paused"}},
    ]}

    response = post_json(api, URL, body)

    assert response.status_code == 422
    assert set(response.json()["error"]["details"]) == {"events[2].data.status"}
    assert Account.objects.get(id=1).balance == 83810
    assert not Account.objects.filter(id=50).exists()


@pytest.mark.django_db
def test_events_feed(api, loaded):
    post_json(api, URL, {"events": [{"entity": "account", "op": "delete", "data": {"id": 4}}]})  # 3 suspended services

    response = api.get(URL, {"accountId": 4, "pageSize": 2})

    body = response.json()
    assert body["total"] == 3
    assert len(body["items"]) == 2
    assert {i["toStatus"] for i in body["items"]} == {"deleted"}
    assert set(body["items"][0]) == {"id", "serviceId", "accountId", "fromStatus", "toStatus", "occurredAt", "source"}
    assert body["items"][0]["occurredAt"].endswith(".000Z")
    assert api.get(URL, {"pageSize": 500}).status_code == 400
