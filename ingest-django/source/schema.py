"""OpenAPI description of /api/v1/source/* (drf-spectacular), named like the shared openapi.yml.

Only documentation lives here: request validation stays in serializers.py and the services.
"""

from drf_spectacular.extensions import OpenApiAuthenticationExtension
from drf_spectacular.utils import OpenApiResponse, extend_schema_field, extend_schema_serializer, inline_serializer
from rest_framework import serializers

from .models import EVENT_SOURCES
from .serializers import AccountDTO, ServiceDTO, TransactionDTO


class HeaderUserScheme(OpenApiAuthenticationExtension):
    """Clients authenticate with the core session cookie; nginx turns it into X-Auth-User."""

    target_class = "source.auth.HeaderUserAuthentication"
    name = "cookieAuth"

    def get_security_definition(self, auto_schema):
        return {"type": "apiKey", "in": "cookie", "name": "prolongo_session"}


@extend_schema_serializer(component_name="Error")
class ErrorSchema(serializers.Serializer):
    error = inline_serializer(
        "ErrorBody",
        {
            "code": serializers.CharField(),
            "message": serializers.CharField(),
            "details": serializers.DictField(child=serializers.ListField(child=serializers.CharField()), required=False),
        },
    )


@extend_schema_serializer(component_name="SnapshotRequest")
class SnapshotRequestSchema(serializers.Serializer):
    accounts = AccountDTO(many=True)
    services = ServiceDTO(many=True)
    transactions = TransactionDTO(many=True)


@extend_schema_field({"type": "string", "format": "binary"})
class UploadField(serializers.FileField):
    pass


class SnapshotUploadSchema(serializers.Serializer):
    file = UploadField(help_text="JSON-файл в формате data.json")


class RemovedSchema(serializers.Serializer):
    accounts = serializers.IntegerField()
    services = serializers.IntegerField()
    transactions = serializers.IntegerField()


@extend_schema_serializer(component_name="SnapshotResponse")
class SnapshotResponseSchema(serializers.Serializer):
    batchId = serializers.IntegerField()
    accounts = serializers.IntegerField()
    services = serializers.IntegerField()
    transactions = serializers.IntegerField()
    statusChanges = serializers.IntegerField()
    removed = RemovedSchema()


@extend_schema_serializer(component_name="SourceEvent")
class SourceEventSchema(serializers.Serializer):
    entity = serializers.ChoiceField(["account", "service", "transaction"])
    op = serializers.ChoiceField(["upsert", "delete"])
    data = serializers.DictField()


@extend_schema_serializer(component_name="EventsRequest")
class EventsRequestSchema(serializers.Serializer):
    at = serializers.DateTimeField(required=False)
    events = serializers.ListField(child=SourceEventSchema(), min_length=1)


@extend_schema_serializer(component_name="EventsResponse")
class EventsResponseSchema(serializers.Serializer):
    applied = serializers.IntegerField()
    statusChanges = serializers.IntegerField()


@extend_schema_serializer(component_name="StatusEvent")
class StatusEventSchema(serializers.Serializer):
    id = serializers.IntegerField()
    serviceId = serializers.IntegerField()
    accountId = serializers.IntegerField()
    fromStatus = serializers.ChoiceField(ServiceDTO().fields["status"].choices, allow_null=True)
    toStatus = serializers.ChoiceField(ServiceDTO().fields["status"].choices)
    occurredAt = serializers.DateTimeField()
    source = serializers.ChoiceField(EVENT_SOURCES)


@extend_schema_serializer(component_name="StatusEventList")
class StatusEventListSchema(serializers.Serializer):
    items = StatusEventSchema(many=True)
    total = serializers.IntegerField()


@extend_schema_serializer(component_name="SimulatorState")
class SimulatorStateSchema(serializers.Serializer):
    running = serializers.BooleanField()
    intervalSeconds = serializers.IntegerField()
    tickHours = serializers.FloatField()
    eventsGenerated = serializers.IntegerField()
    dataTime = serializers.DateTimeField()


def error(description):
    return OpenApiResponse(ErrorSchema, description=description)


UNAUTHORIZED = error("Нет сессии или она истекла")
VALIDATION = error("Ошибка валидации; details — пути к полям")
BAD_REQUEST = error("Неверный запрос")
