import json
from datetime import timezone

from django.conf import settings
from drf_spectacular.utils import OpenApiResponse, extend_schema, extend_schema_serializer
from rest_framework import serializers
from rest_framework.exceptions import ParseError
from rest_framework.parsers import JSONParser, MultiPartParser
from rest_framework.response import Response
from rest_framework.views import APIView

from . import schema
from .exceptions import BadRequest
from .models import EVENT_SOURCES, SimulatorState, StatusEvent
from .services.events import apply_events
from .services.simulator import current_data_time, get_state, run_tick
from .services.snapshot import apply_snapshot


def iso_z(value):
    """Contract date format: 2026-10-19T15:05:00.000Z."""
    return value.astimezone(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


class SnapshotView(APIView):
    """PUT /api/v1/source/snapshot: replace all source data with a data.json snapshot."""

    parser_classes = [JSONParser, MultiPartParser]

    @extend_schema(
        operation_id="putSnapshot",
        tags=["source"],
        summary="Загрузка или полная замена снимка данных",
        request={"application/json": schema.SnapshotRequestSchema, "multipart/form-data": schema.SnapshotUploadSchema},
        responses={
            200: OpenApiResponse(schema.SnapshotResponseSchema, description="Снимок применён"),
            400: schema.error("Нет поля file, файл не JSON или больше 20 МБ"),
            401: schema.UNAUTHORIZED,
            422: schema.VALIDATION,
        },
    )
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


@extend_schema_serializer(component_name="EventsQuery")
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

    @extend_schema(
        operation_id="listSourceEvents",
        tags=["source"],
        summary="Лента смен статусов услуг",
        parameters=[EventsQueryDTO],
        responses={
            200: OpenApiResponse(schema.StatusEventListSchema, description="События, новые сверху"),
            400: schema.BAD_REQUEST,
            401: schema.UNAUTHORIZED,
        },
    )
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

    @extend_schema(
        operation_id="postSourceEvents",
        tags=["source"],
        summary="Точечные изменения данных",
        request=schema.EventsRequestSchema,
        responses={
            200: OpenApiResponse(schema.EventsResponseSchema, description="Все события применены в одной транзакции"),
            400: schema.error("Тело не JSON"),
            401: schema.UNAUTHORIZED,
            422: schema.VALIDATION,
        },
    )
    def post(self, request):
        return Response(apply_events(request.data))


@extend_schema_serializer(component_name="SimulatorAction")
class SimulatorActionDTO(serializers.Serializer):
    action = serializers.ChoiceField(
        ["start", "stop", "tick"], error_messages={"invalid_choice": "Недопустимое значение: {input}"}
    )
    intervalSeconds = serializers.IntegerField(required=False, min_value=1, max_value=60)
    tickHours = serializers.FloatField(required=False, min_value=0.25, max_value=168)


def simulator_payload(state):
    return {
        "running": state.running,
        "intervalSeconds": state.interval_seconds,
        "tickHours": state.tick_hours,
        "eventsGenerated": state.events_generated,
        "dataTime": iso_z(current_data_time()),
    }


class SimulatorView(APIView):
    """GET: simulator state. POST: start / stop / one tick, optionally with new interval and tick size."""

    parser_classes = [JSONParser]

    @extend_schema(
        operation_id="getSimulator",
        tags=["source"],
        summary="Состояние симулятора",
        responses={200: OpenApiResponse(schema.SimulatorStateSchema, description="Текущее состояние"), 401: schema.UNAUTHORIZED},
    )
    def get(self, request):
        return Response(simulator_payload(get_state()))

    @extend_schema(
        operation_id="controlSimulator",
        tags=["source"],
        summary="Запуск, остановка или один шаг симулятора",
        request=SimulatorActionDTO,
        responses={
            200: OpenApiResponse(schema.SimulatorStateSchema, description="Состояние после действия"),
            400: schema.error("Тело не JSON"),
            401: schema.UNAUTHORIZED,
            422: schema.VALIDATION,
        },
    )
    def post(self, request):
        command = SimulatorActionDTO(data=request.data)
        command.is_valid(raise_exception=True)
        params = command.validated_data

        get_state()  # make sure the row exists before locking it
        state = SimulatorState.objects.select_for_update().get(id=1)
        if "intervalSeconds" in params:
            state.interval_seconds = params["intervalSeconds"]
        if "tickHours" in params:
            state.tick_hours = params["tickHours"]
        if params["action"] in ("start", "stop"):
            state.running = params["action"] == "start"
        state.save()

        if params["action"] == "tick":
            run_tick()
        return Response(simulator_payload(get_state()))
