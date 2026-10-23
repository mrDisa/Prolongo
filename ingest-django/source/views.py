import json

from django.conf import settings
from rest_framework.parsers import JSONParser, MultiPartParser
from rest_framework.response import Response
from rest_framework.views import APIView

from .exceptions import ValidationFailed, error_response
from .services.snapshot import apply_snapshot


class SnapshotView(APIView):
    """PUT /api/v1/source/snapshot: replace all source data with a data.json snapshot."""

    parser_classes = [JSONParser, MultiPartParser]

    def put(self, request):
        if request.content_type.startswith("multipart/form-data"):
            upload = request.FILES.get("file")
            if upload is None:
                return error_response(400, "bad_request", "Нужен файл в поле file")
            if upload.size > settings.DATA_UPLOAD_MAX_MEMORY_SIZE:
                return error_response(400, "bad_request", "Файл больше 20 МБ")
            try:
                payload = json.load(upload)
            except (UnicodeDecodeError, json.JSONDecodeError):
                return error_response(400, "bad_request", "Файл не является корректным JSON")
        else:
            payload = request.data

        try:
            result = apply_snapshot(payload, source="snapshot")
        except ValidationFailed as exc:
            return error_response(422, "validation_error", exc.message, exc.details)
        return Response(result)
