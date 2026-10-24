import copy
import json

import pytest
from django.test.client import BOUNDARY, MULTIPART_CONTENT, encode_multipart

from source.models import Account, Service, SourceClock, StatusEvent, Transaction

from .conftest import put_json

URL = "/api/v1/source/snapshot"


def _counts():
    return Account.objects.count(), Service.objects.count(), Transaction.objects.count()


@pytest.mark.django_db
def test_snapshot_load(api, data_json):
    response = put_json(api, URL, data_json)

    assert response.status_code == 200
    body = response.json()
    assert (body["accounts"], body["services"], body["transactions"]) == (10, 30, 100)
    assert body["removed"] == {"accounts": 0, "services": 0, "transactions": 0}
    assert _counts() == (10, 30, 100)
    assert SourceClock.objects.get().data_time.isoformat() == "2026-10-19T14:35:53+00:00"
    assert StatusEvent.objects.count() == 0  # initial inserts are not status changes


@pytest.mark.django_db
def test_snapshot_idempotent(api, loaded):
    response = put_json(api, URL, loaded)

    assert response.status_code == 200
    assert response.json()["statusChanges"] == 0
    assert _counts() == (10, 30, 100)


@pytest.mark.django_db
def test_snapshot_diff_status_change(api, loaded):
    changed = copy.deepcopy(loaded)
    changed["services"][0]["status"] = "prolong_wait"  # service 1 is active

    response = put_json(api, URL, changed)

    assert response.json()["statusChanges"] == 1
    event = StatusEvent.objects.get(batch_id=response.json()["batchId"])
    assert (event.service_id, event.from_status, event.to_status, event.source) == (1, "active", "prolong_wait", "snapshot")


@pytest.mark.django_db
def test_snapshot_diff_removed_live_service(api, loaded):
    removed = copy.deepcopy(loaded)
    removed["services"] = [s for s in removed["services"] if s["id"] != 6]  # active
    removed["transactions"] = [t for t in removed["transactions"] if t["serviceId"] != 6]

    response = put_json(api, URL, removed)

    body = response.json()
    assert body["removed"]["services"] == 1
    assert body["removed"]["transactions"] == 4
    event = StatusEvent.objects.get(batch_id=body["batchId"])
    assert (event.service_id, event.from_status, event.to_status) == (6, "active", "deleted")


@pytest.mark.django_db
def test_snapshot_validation(api, loaded):
    bad = copy.deepcopy(loaded)
    bad["services"][3]["status"] = "paused"
    bad["services"][4]["ownerId"] = 999
    bad["transactions"][7]["amount"] = 5000

    response = put_json(api, URL, bad)

    assert response.status_code == 422
    error = response.json()["error"]
    assert error["code"] == "validation_error"
    assert set(error["details"]) == {"services[3].status", "services[4].ownerId", "transactions[7].amount"}
    assert error["details"]["services[3].status"] == ["Недопустимое значение: paused"]
    assert _counts() == (10, 30, 100)  # nothing applied


@pytest.mark.django_db
def test_snapshot_multipart_file(api, data_json, tmp_path):
    path = tmp_path / "data.json"
    path.write_text(json.dumps(data_json))
    with path.open("rb") as f:
        response = api.generic("PUT", URL, encode_multipart(BOUNDARY, {"file": f}), MULTIPART_CONTENT)

    assert response.status_code == 200
    assert response.json()["services"] == 30
