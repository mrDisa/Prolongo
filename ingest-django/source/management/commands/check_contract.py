"""Compare the generated ingest schema with /api/v1/source/* in the shared openapi.yml."""

from pathlib import Path

import yaml
from django.conf import settings
from django.core.management.base import BaseCommand, CommandError
from drf_spectacular.generators import SchemaGenerator

PREFIX = "/api/v1/source/"
METHODS = ("get", "put", "post", "patch", "delete")
BIGINT_MAX = 2**63 - 1
COMPARED_KEYS = ("type", "format", "pattern", "minimum", "maximum", "minItems")


def _resolve(spec, node):
    while isinstance(node, dict) and "$ref" in node:
        name = node["$ref"].rsplit("/", 1)[-1]
        section = node["$ref"].split("/")[2]
        node = spec["components"][section][name]
    return node


def _is_null(spec, option):
    option = _resolve(spec, option)
    return option.get("type") == "null" or option.get("enum") == [None]


def normalize(spec, node):
    """Reduce a schema to what matters for clients: shape, types, enums, bounds, required fields."""
    node = _resolve(spec, node)
    nullable = bool(node.get("nullable"))
    if "allOf" in node and len(node["allOf"]) == 1:
        return {**normalize(spec, node["allOf"][0]), **({"nullable": True} if nullable else {})}
    for key in ("oneOf", "anyOf"):
        if key in node:
            options = [o for o in node[key] if not _is_null(spec, o)]
            if len(options) == 1:
                return {**normalize(spec, options[0]), "nullable": True}
    out = {}
    node_type = node.get("type")
    if isinstance(node_type, list):  # OpenAPI 3.1: ["string", "null"]
        nullable = nullable or "null" in node_type
        node_type = [t for t in node_type if t != "null"][0]
    if node_type:
        out["type"] = node_type
    for key in COMPARED_KEYS[1:]:
        if key in node and not (key == "maximum" and node[key] >= BIGINT_MAX):
            out[key] = node[key]
    if node_type in ("integer", "number"):
        out.pop("format", None)  # int64 / double only refine the type
    if "enum" in node:
        out["enum"] = sorted(str(v) for v in node["enum"] if v is not None)
        nullable = nullable or None in node["enum"]
    if nullable:
        out["nullable"] = True
    if "properties" in node:
        out["properties"] = {k: normalize(spec, v) for k, v in sorted(node["properties"].items())}
        out["required"] = sorted(node.get("required", []))
    if "items" in node:
        out["items"] = normalize(spec, node["items"])
    return out


def diff(a, b, path):
    """Yield human-readable differences between two normalized schemas (a = contract, b = Django)."""
    if isinstance(a, dict) and isinstance(b, dict):
        for key in sorted(set(a) | set(b)):
            if key not in b:
                yield f"{path}.{key}: only in openapi.yml = {a[key]}"
            elif key not in a:
                yield f"{path}.{key}: only in Django = {b[key]}"
            else:
                yield from diff(a[key], b[key], f"{path}.{key}")
    elif a != b:
        yield f"{path}: openapi.yml = {a} / Django = {b}"


def operations(spec):
    for path, item in spec["paths"].items():
        if path.startswith(PREFIX):
            for method in METHODS:
                if method in item:
                    yield (path, method), item[method]


def describe(spec, op):
    params = {
        p["name"]: normalize(spec, p.get("schema", {}))
        for p in (_resolve(spec, p) for p in op.get("parameters", []))
        if p.get("in") == "query"
    }
    body = op.get("requestBody", {}).get("content", {})
    ok = _resolve(spec, op.get("responses", {}).get("200", {})).get("content", {}).get("application/json", {})
    return {
        "operationId": op.get("operationId"),
        "responses": sorted(op.get("responses", {})),
        "query": params,
        "request": {ctype: normalize(spec, c.get("schema", {})) for ctype, c in body.items()},
        "response200": normalize(spec, ok.get("schema", {})),
    }


class Command(BaseCommand):
    help = "Check that the ingest API schema matches /api/v1/source/* in openapi.yml."

    def add_arguments(self, parser):
        parser.add_argument("--contract", default=str(Path(settings.BASE_DIR).parent / "openapi.yml"))

    def handle(self, *args, contract, **options):
        contract_spec = yaml.safe_load(Path(contract).read_text())
        django_spec = SchemaGenerator().get_schema(request=None, public=True)

        theirs = dict(operations(contract_spec))
        ours = dict(operations(django_spec))
        problems = []
        for key in sorted(set(theirs) | set(ours)):
            label = f"{key[1].upper()} {key[0]}"
            if key not in ours:
                problems.append(f"{label}: only in openapi.yml")
            elif key not in theirs:
                problems.append(f"{label}: only in Django")
            else:
                a, b = describe(contract_spec, theirs[key]), describe(django_spec, ours[key])
                problems += [f"{label} {line}" for line in diff(a, b, "")]

        for line in problems:
            self.stdout.write(line)
        if problems:
            raise CommandError(f"{len(problems)} difference(s) with {contract}")
        self.stdout.write(self.style.SUCCESS(f"{len(ours)} operations match {contract}"))
