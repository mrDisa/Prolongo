import pytest
from django.contrib.auth.models import User
from django.test import Client

from source.models import Account, StatusEvent

ENDPOINTS = [
    ("put", "/api/v1/source/snapshot"),
    ("get", "/api/v1/source/events"),
    ("post", "/api/v1/source/events"),
    ("get", "/api/v1/source/simulator"),
    ("post", "/api/v1/source/simulator"),
]


@pytest.mark.django_db
@pytest.mark.parametrize("method,url", ENDPOINTS)
@pytest.mark.parametrize("headers", [{}, {"HTTP_X_AUTH_USER": ""}])
def test_auth(method, url, headers):
    client = Client()
    if method == "get":
        response = client.get(url, **headers)
    else:
        response = getattr(client, method)(url, "{}", content_type="application/json", **headers)

    assert response.status_code == 401
    assert response.json() == {"error": {"code": "unauthorized", "message": "Требуется вход"}}


@pytest.mark.django_db
def test_admin_single_sign_on():
    assert Client().get("/admin/").status_code == 302  # no header: Django login page

    response = Client(HTTP_X_AUTH_USER="marketer").get("/admin/")

    assert response.status_code == 200
    user = User.objects.get(username="marketer")
    assert user.is_staff and user.is_superuser


@pytest.mark.django_db
def test_admin_changes_are_labelled_admin(loaded):
    client = Client(HTTP_X_AUTH_USER="admin")
    form = {"form-TOTAL_FORMS": "1", "form-INITIAL_FORMS": "1", "form-0-id": "5", "form-0-status": "active",
            "_save": "Сохранить"}

    response = client.post("/admin/source/service/?id__exact=5", form)

    assert response.status_code == 302
    event = StatusEvent.objects.get(service_id=5)
    assert (event.from_status, event.to_status, event.source) == ("prolong_wait", "active", "admin")
    assert Account.objects.count() == 10
