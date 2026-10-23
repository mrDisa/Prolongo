import json

from django.core.management.base import BaseCommand, CommandError

from source.exceptions import ValidationFailed
from source.models import Account
from source.services.snapshot import apply_snapshot


class Command(BaseCommand):
    help = "Load a data.json snapshot (organizers' format) as the initial dataset."

    def add_arguments(self, parser):
        parser.add_argument("path", help="Path to data.json")
        parser.add_argument("--if-empty", action="store_true", help="Skip when public.accounts already has rows")

    def handle(self, *args, path, if_empty, **options):
        if if_empty and Account.objects.exists():
            self.stdout.write("Dataset already loaded, skipping.")
            return
        try:
            with open(path, encoding="utf-8") as f:
                payload = json.load(f)
        except (OSError, json.JSONDecodeError) as exc:
            raise CommandError(f"Cannot read {path}: {exc}") from exc
        try:
            result = apply_snapshot(payload, source="seed")
        except ValidationFailed as exc:
            raise CommandError(f"{exc.message}: {json.dumps(exc.details, ensure_ascii=False)}") from exc
        self.stdout.write(self.style.SUCCESS(f"Loaded {path}: {json.dumps(result)}"))
