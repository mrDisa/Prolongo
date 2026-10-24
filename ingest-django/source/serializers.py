"""DTOs in the organizers' data.json format (camelCase).

Validation mirrors the SQL constraints of 001_schema.up.sql, so bad input
gets a readable 422 with paths instead of a database error.
"""

from drf_spectacular.utils import extend_schema_serializer
from rest_framework import serializers

from .models import BONUS_STATES, PAYMENT_TERMS, SERVICE_STATUSES, TX_STATUSES, TX_TYPES

BIGINT_MAX = 2**63 - 1
INVALID_CHOICE = {"invalid_choice": "Недопустимое значение: {input}"}


def _choices(pairs):
    return [value for value, _ in pairs]


def _money(**kwargs):
    return serializers.IntegerField(min_value=0, max_value=BIGINT_MAX, **kwargs)


def _id():
    return serializers.IntegerField(min_value=1, max_value=BIGINT_MAX)


@extend_schema_serializer(component_name="SourceAccount")
class AccountDTO(serializers.Serializer):
    id = _id()
    email = serializers.CharField(trim_whitespace=False)
    balance = _money(required=False, default=0)
    withdrawBalance = _money(required=False, default=0)
    totalReplenished = _money(required=False, default=0)
    bonusBalance = _money(required=False, default=0)
    bonusState = serializers.ChoiceField(
        _choices(BONUS_STATES), required=False, default="locked", error_messages=INVALID_CHOICE
    )


@extend_schema_serializer(component_name="SourceService")
class ServiceDTO(serializers.Serializer):
    id = _id()
    # \Z instead of $: in Python "$" also matches before a trailing newline, in PostgreSQL it does not.
    name = serializers.RegexField(
        r"^[A-Za-z0-9-]+\Z",
        trim_whitespace=False,
        error_messages={"invalid": "Допустимы только латинские буквы, цифры и дефис"},
    )
    ownerId = _id()
    paymentTerm = serializers.ChoiceField(_choices(PAYMENT_TERMS), error_messages=INVALID_CHOICE)
    autoProlong = serializers.BooleanField(required=False, default=False)
    createdAt = serializers.DateTimeField()
    expiresAt = serializers.DateTimeField()
    status = serializers.ChoiceField(_choices(SERVICE_STATUSES), error_messages=INVALID_CHOICE)
    price = _money()

    def validate(self, attrs):
        if attrs["expiresAt"] < attrs["createdAt"]:
            raise serializers.ValidationError({"expiresAt": ["Должно быть не раньше createdAt"]})
        return attrs


@extend_schema_serializer(component_name="SourceTransaction")
class TransactionDTO(serializers.Serializer):
    id = _id()
    amount = serializers.IntegerField(min_value=0, max_value=1000)
    status = serializers.ChoiceField(
        _choices(TX_STATUSES), required=False, default="not_performed", error_messages=INVALID_CHOICE
    )
    performedAt = serializers.DateTimeField(required=False, allow_null=True, default=None)
    createdAt = serializers.DateTimeField()
    type = serializers.ChoiceField(_choices(TX_TYPES), error_messages=INVALID_CHOICE)
    serviceId = _id()

    def validate(self, attrs):
        performed_at = attrs.get("performedAt")
        if attrs["status"] == "not_performed" and performed_at is not None:
            raise serializers.ValidationError({"performedAt": ["У not_performed должно быть null"]})
        if attrs["status"] == "performed":
            if performed_at is None:
                raise serializers.ValidationError({"performedAt": ["У performed обязательно"]})
            if performed_at < attrs["createdAt"]:
                raise serializers.ValidationError({"performedAt": ["Должно быть не раньше createdAt"]})
        return attrs


def _is_int(value):
    return isinstance(value, int) and not isinstance(value, bool)


def _validate_items(dto_class, prefix, items, errors):
    """Validate each item separately so every error keeps its index in the path."""
    valid = []
    for index, item in enumerate(items):
        dto = dto_class(data=item)
        if dto.is_valid():
            valid.append(dict(dto.validated_data))
            continue
        for field, messages in dto.errors.items():
            path = f"{prefix}[{index}]" if field == "non_field_errors" else f"{prefix}[{index}].{field}"
            errors.setdefault(path, []).extend(str(m) for m in messages)
    return valid


def _check_unique(prefix, field, items, errors):
    seen = set()
    for index, item in enumerate(items):
        value = item.get(field) if isinstance(item, dict) else None
        if value is None or not isinstance(value, (int, str)):
            continue
        if value in seen:
            errors.setdefault(f"{prefix}[{index}].{field}", []).append(f"Повторяется в снимке: {value}")
        seen.add(value)


def _check_refs(prefix, field, items, known_ids, target, errors):
    for index, item in enumerate(items):
        value = item.get(field) if isinstance(item, dict) else None
        if _is_int(value) and value not in known_ids:
            errors.setdefault(f"{prefix}[{index}].{field}", []).append(f"Нет среди {target} снимка: {value}")


def _raw_ids(items):
    return {item["id"] for item in items if isinstance(item, dict) and _is_int(item.get("id"))}


def validate_snapshot(payload):
    """Return (data, errors). data holds validated accounts / services / transactions;
    errors maps paths like "services[3].status" to lists of messages."""
    errors = {}
    if not isinstance(payload, dict):
        return None, {"body": ["Ожидается объект с полями accounts, services, transactions"]}

    collections = {}
    for key in ("accounts", "services", "transactions"):
        value = payload.get(key)
        if value is None:
            errors[key] = ["Обязательное поле."]
        elif not isinstance(value, list):
            errors[key] = ["Ожидается список."]
        else:
            collections[key] = value
    if errors:
        return None, errors

    accounts, services, transactions = (collections[k] for k in ("accounts", "services", "transactions"))
    data = {
        "accounts": _validate_items(AccountDTO, "accounts", accounts, errors),
        "services": _validate_items(ServiceDTO, "services", services, errors),
        "transactions": _validate_items(TransactionDTO, "transactions", transactions, errors),
    }

    _check_unique("accounts", "id", accounts, errors)
    _check_unique("accounts", "email", accounts, errors)
    _check_unique("services", "id", services, errors)
    _check_unique("transactions", "id", transactions, errors)
    _check_refs("services", "ownerId", services, _raw_ids(accounts), "accounts", errors)
    _check_refs("transactions", "serviceId", transactions, _raw_ids(services), "services", errors)

    return (None, errors) if errors else (data, {})
