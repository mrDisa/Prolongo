import copy
import json
from pathlib import Path

import pytest
from django.conf import settings
from django.test import Client

from source.services.snapshot import apply_snapshot

# Organizers' files: ../data locally, /app/data inside the ingest container (compose volume).
DATA_DIRS = [Path(settings.BASE_DIR).parent / "data" / "test-data", Path(settings.BASE_DIR) / "data" / "test-data"]


def organizers_file(name):
    for directory in DATA_DIRS:
        if (directory / name).exists():
            return directory / name
    pytest.skip(f"{name} of the organizers is not available")


@pytest.fixture
def data_json():
    return json.loads(organizers_file("data.json").read_text())


@pytest.fixture
def loaded(db, data_json):
    """The organizers' dataset loaded as at the first start."""
    apply_snapshot(copy.deepcopy(data_json), source="seed")
    return data_json


@pytest.fixture
def api():
    """A client that passed core auth: nginx adds X-Auth-User."""
    return Client(HTTP_X_AUTH_USER="admin")


def put_json(client, url, body):
    return client.put(url, json.dumps(body), content_type="application/json")


def post_json(client, url, body):
    return client.post(url, json.dumps(body), content_type="application/json")
