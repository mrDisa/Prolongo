import json
from datetime import timezone

from django.conf import settings
from rest_framework import serializers
from rest_framework.exceptions import ParseError
from rest_framework.parsers import JSONParser, MultiPartParser
from rest_framework.response import Response
from rest_framework.views import APIView

from .exceptions import BadRequest
from .models import StatusEvent
from .services.events import apply_events
from .services.snapshot import apply_snapshot

EVENT_SOURCES = ["seed", "snapshot", "event", "admin", "simulator", "sql", "demo_backfill"]


def iso_z(value):
    """Contract date format: 2026-10-19T15:05:00.000Z."""
    return value.astimezone(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


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


class EventsQueryDTO(serializers.Serializer):
    accountId = serializers.IntegerField(required=False, min_value=1)
    serviceId = serializers.IntegerField(required=False, min_value=1)
    source = serializers.ChoiceField(
        EVENT_SOURCES, required=False, error_messages={"invalid_choice": "Недопустимое значение: {input}"}
    )
    page = serializers.IntegerField(required=False, min_value=1, default=1)
    pageSize = serializers.IntegerField(required=False, min_value=1, max_value=200, default=50)


class EventsView(APIView):
    """GET: feed of service status changes. POST: point changes to the source data."""

    parser_classes = [JSONParser]

    def get(self, request):
        query = EventsQueryDTO(data=request.query_params)
        if not query.is_valid():
            raise BadRequest("; ".join(f"{field}: {messages[0]}" for field, messages in query.errors.items()))
        params = query.validated_data

        events = StatusEvent.objects.all()
        if "accountId" in params:
            events = events.filter(account_id=params["accountId"])
        if "serviceId" in params:
            events = events.filter(service_id=params["serviceId"])
        if "source" in params:
            events = events.filter(source=params["source"])

        total = events.count()
        start = (params["page"] - 1) * params["pageSize"]
        page = events.order_by("-occurred_at", "-id")[start : start + params["pageSize"]]
        items = [
            {
                "id": e.id,
                "serviceId": e.service_id,
                "accountId": e.account_id,
                "fromStatus": e.from_status,
                "toStatus": e.to_status,
                "occurredAt": iso_z(e.occurred_at),
                "source": e.source,
            }
            for e in page
        ]
        return Response({"items": items, "total": total})

    def post(self, request):
        return Response(apply_events(request.data))
