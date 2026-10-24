"""One error format for the whole API: {"error": {"code", "message", "details"}} (ОБЩЕЕ_ТЗ §7.2)."""

import logging

from django.core.exceptions import PermissionDenied
from django.http import Http404, JsonResponse
from rest_framework import exceptions
from rest_framework.response import Response
from rest_framework.views import set_rollback

logger = logging.getLogger(__name__)

UNAUTHORIZED_MESSAGE = "Требуется вход"
NOT_FOUND_MESSAGE = "Маршрут не найден"
INTERNAL_MESSAGE = "Внутренняя ошибка сервера"


class ValidationFailed(Exception):
    """Input does not satisfy the source schema; details map paths to messages."""

    def __init__(self, details, message="Данные не прошли проверку"):
        super().__init__(message)
        self.message = message
        self.details = details


class BadRequest(exceptions.APIException):
    """Invalid query parameters: 400 bad_request with a readable message."""

    status_code = 400
    default_code = "bad_request"
    default_detail = "Неверные параметры запроса"


def error_body(code, message, details=None):
    error = {"code": code, "message": message}
    if details is not None:
        error["details"] = details
    return {"error": error}


def error_response(status, code, message, details=None, headers=None):
    return Response(error_body(code, message, details), status=status, headers=headers)


def _messages(value):
    if isinstance(value, (list, tuple)):
        return [str(v) for v in value]
    return [str(value)]


def _validation_details(detail):
    if isinstance(detail, dict):
        return {str(field): _messages(value) for field, value in detail.items()}
    return {"body": _messages(detail)}


def api_exception_handler(exc, context):
    """DRF EXCEPTION_HANDLER: every error, including unexpected ones, in the contract format."""
    set_rollback()

    if isinstance(exc, ValidationFailed):
        return error_response(422, "validation_error", exc.message, exc.details)
    if isinstance(exc, Http404):
        exc = exceptions.NotFound()
    elif isinstance(exc, PermissionDenied):
        exc = exceptions.PermissionDenied()

    if isinstance(exc, exceptions.ValidationError):
        return error_response(422, "validation_error", "Данные не прошли проверку", _validation_details(exc.detail))
    if isinstance(exc, (exceptions.NotAuthenticated, exceptions.AuthenticationFailed)):
        auth_header = getattr(exc, "auth_header", None)
        headers = {"WWW-Authenticate": auth_header} if auth_header else None
        return error_response(401, "unauthorized", UNAUTHORIZED_MESSAGE, headers=headers)
    if isinstance(exc, exceptions.ParseError):
        return error_response(400, "bad_request", f"Некорректное тело запроса: {exc.detail}")
    if isinstance(exc, exceptions.UnsupportedMediaType):
        return error_response(400, "bad_request", "Ожидается application/json или multipart/form-data")
    if isinstance(exc, exceptions.NotFound):
        return error_response(404, "not_found", "Не найдено")
    if isinstance(exc, exceptions.MethodNotAllowed):
        return error_response(405, "method_not_allowed", "Метод не поддерживается")
    if isinstance(exc, exceptions.APIException):
        return error_response(exc.status_code, exc.default_code, str(exc.detail))

    logger.exception("Unhandled error in %s", context.get("view").__class__.__name__)
    return error_response(500, "internal", INTERNAL_MESSAGE)


# Django-level handlers (outside DRF views): JSON for the API, Django defaults elsewhere.


def _json_error(status, code, message):
    return JsonResponse(error_body(code, message), status=status, json_dumps_params={"ensure_ascii": False})


def handler404(request, exception):
    if request.path.startswith("/api/"):
        return _json_error(404, "not_found", NOT_FOUND_MESSAGE)
    from django.views.defaults import page_not_found

    return page_not_found(request, exception)


def handler500(request):
    if request.path.startswith("/api/"):
        return _json_error(500, "internal", INTERNAL_MESSAGE)
    from django.views.defaults import server_error

    return server_error(request)
