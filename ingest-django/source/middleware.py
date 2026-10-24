from django.contrib.auth.middleware import RemoteUserMiddleware
from django.db import connection, transaction

from .services.db import set_context


def _is_admin(request):
    return request.path.startswith("/admin/")


class AdminRemoteUserMiddleware(RemoteUserMiddleware):
    """Single sign-on for /admin/: the user comes from X-Auth-User set by nginx after core auth.

    Limited to /admin/ so API requests do not create Django sessions.
    """

    header = "HTTP_X_AUTH_USER"

    def process_request(self, request):
        if _is_admin(request):
            super().process_request(request)


class AdminSourceMiddleware:
    """Runs each admin request in one transaction labelled source=admin for the status audit trigger.

    set_context settings live until the end of the transaction, so the transaction is opened here.
    """

    def __init__(self, get_response):
        self.get_response = get_response

    def __call__(self, request):
        if not _is_admin(request):
            return self.get_response(request)
        with transaction.atomic():
            with connection.cursor() as cursor:
                set_context(cursor, source="admin")
            return self.get_response(request)
