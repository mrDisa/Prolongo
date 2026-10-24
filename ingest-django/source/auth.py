"""Access to ingest is decided by core: nginx calls auth_request and passes X-Auth-User.

nginx overwrites a client-supplied X-Auth-User, and port 8001 is not published,
so the header can be trusted.
"""

from rest_framework.authentication import BaseAuthentication


class HeaderUser:
    """A panel user authenticated by core; ingest keeps no user records of its own."""

    is_authenticated = True
    is_anonymous = False

    def __init__(self, username):
        self.username = username

    def __str__(self):
        return self.username


class HeaderUserAuthentication(BaseAuthentication):
    def authenticate(self, request):
        username = request.META.get("HTTP_X_AUTH_USER", "").strip()
        if not username:
            return None
        return HeaderUser(username), None

    def authenticate_header(self, request):
        # Any non-empty value makes DRF answer 401 instead of 403.
        return "Session"
