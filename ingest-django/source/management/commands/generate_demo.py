import json

from django.core.management.base import BaseCommand

from source.services.demo import generate_demo


class Command(BaseCommand):
    help = "Add demo accounts with realistic churn scenarios and 90 days of status history (skipped if already added)."

    def add_arguments(self, parser):
        parser.add_argument("--accounts", type=int, default=300)
        parser.add_argument("--seed", type=int, default=7)
        parser.add_argument("--backfill-days", type=int, default=90)

    def handle(self, *args, accounts, seed, backfill_days, **options):
        result = generate_demo(accounts=accounts, seed=seed, backfill_days=backfill_days)
        if result.get("skipped"):
            self.stdout.write("Demo data already present (demo- emails), skipping.")
            return
        self.stdout.write(self.style.SUCCESS(f"Demo data added: {json.dumps(result)}"))
