from unittest import mock

import pytest

from source.models import Account, Service, SourceClock, StatusEvent, Transaction
from source.services.simulator import Tick, run_tick
from source.services.snapshot import apply_snapshot

from .conftest import post_json


def _account(i, balance, bonus=0, state="locked"):
    return {"id": i, "email": f"a{i}@example.test", "balance": balance, "totalReplenished": balance,
            "bonusBalance": bonus, "bonusState": state}


def _service(i, owner, term, auto, expires, status, price):
    return {"id": i, "name": f"s-{i}", "ownerId": owner, "paymentTerm": term, "autoProlong": auto,
            "createdAt": "2026-09-01T00:00:00Z", "expiresAt": expires, "status": status, "price": price}


@pytest.fixture
def tick_data(db):
    apply_snapshot({
        "accounts": [_account(1, 0), _account(2, 10000), _account(3, 100, bonus=1000, state="unlocked")],
        "services": [
            _service(1, 1, "month", True, "2026-10-01T03:00:00Z", "active", 500),         # expired, no money
            _service(2, 1, "week", False, "2026-09-20T00:00:00Z", "prolong_wait", 500),   # waiting > 5 days
            _service(3, 2, "month", True, "2026-10-01T04:00:00Z", "active", 700),         # expired, has money
            _service(4, 3, "month", True, "2026-10-01T04:00:00Z", "active", 800),         # bonuses pay first
            _service(5, 2, "year", True, "2027-09-01T00:00:00Z", "active", 900),          # not expired
        ],
        "transactions": [],
    })
    SourceClock.objects.filter(id=1).update(data_time="2026-10-01T05:30:00Z")


@pytest.mark.django_db
def test_simulator_tick(tick_data):
    with mock.patch.object(Tick, "random_events", lambda self: None):
        result = run_tick()

    assert result["dataTime"].isoformat() == "2026-10-01T11:30:00+00:00"
    status = dict(Service.objects.values_list("id", "status"))
    assert status == {1: "prolong_wait", 2: "deleted", 3: "active", 4: "active", 5: "active"}

    assert list(Transaction.objects.filter(service_id=1).values_list("type", "status")) == [("prolong", "not_performed")]
    assert list(Transaction.objects.filter(service_id=3).values_list("type", "status")) == [("prolong", "performed")]
    assert Service.objects.get(id=3).expires_at.isoformat() == "2026-11-01T04:00:00+00:00"
    assert not Transaction.objects.filter(service_id=5).exists()

    assert Account.objects.get(id=2).balance == 10000 - 700
    a3 = Account.objects.get(id=3)
    assert (a3.bonus_balance, a3.balance) == (200, 100)

    events = set(StatusEvent.objects.values_list("service_id", "from_status", "to_status", "source"))
    assert events == {(1, "active", "prolong_wait", "simulator"), (2, "prolong_wait", "deleted", "simulator")}


@pytest.mark.django_db
def test_simulator_api(api, loaded):
    url = "/api/v1/source/simulator"
    assert api.get(url).json() == {"running": False, "intervalSeconds": 3, "tickHours": 6.0,
                                   "eventsGenerated": 0, "dataTime": "2026-10-19T14:35:53.000Z"}

    assert post_json(api, url, {"action": "start", "intervalSeconds": 2}).json()["running"] is True
    assert post_json(api, url, {"action": "stop"}).json()["running"] is False
    ticked = post_json(api, url, {"action": "tick"}).json()
    assert ticked["dataTime"] == "2026-10-19T20:35:53.000Z"
    assert ticked["eventsGenerated"] > 0

    bad = post_json(api, url, {"action": "pause", "tickHours": 500})
    assert bad.status_code == 422
    assert set(bad.json()["error"]["details"]) == {"action", "tickHours"}
