from rest_framework.response import Response


class ValidationFailed(Exception):
    """Input does not satisfy the source schema; details map paths to messages."""

    def __init__(self, details, message="Данные не прошли проверку"):
        super().__init__(message)
        self.message = message
        self.details = details


def error_response(status, code, message, details=None):
    """Error body from the API contract: {"error": {"code", "message", "details"}}."""
    error = {"code": code, "message": message}
    if details is not None:
        error["details"] = details
    return Response({"error": error}, status=status)
