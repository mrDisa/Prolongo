import json

from django.conf import settings
from rest_framework.exceptions import ParseError
from rest_framework.parsers import JSONParser, MultiPartParser
from rest_framework.response import Response
from rest_framework.views import APIView

from .services.snapshot import apply_snapshot


class SnapshotView(APIView):
    """PUT /api/v1/source/snapshot: replace all source data with a data.json snapshot."""

    parser_classes = [JSONParser, MultiPartParser]

    def put(self, request):
        if request.content_type.startswith("multipart/form-data"):
            upload = request.FILES.get("file")
            if upload is None:
                raise ParseError("нужен файл в поле file")
            if upload.size > settings.DATA_UPLOAD_MAX_MEMORY_SIZE:
                raise ParseError("файл больше 20 МБ")
            try:
                payload = json.load(upload)
            except (UnicodeDecodeError, json.JSONDecodeError) as exc:
                raise ParseError("файл не является корректным JSON") from exc
        else:
            payload = request.data

        return Response(apply_snapshot(payload, source="snapshot"))
