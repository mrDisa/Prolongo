from pathlib import Path

import pytest
from django.conf import settings
from django.core.management import call_command

CONTRACT = Path(settings.BASE_DIR).parent / "openapi.yml"


@pytest.mark.skipif(not CONTRACT.exists(), reason="openapi.yml is not next to ingest-django (e.g. inside the container)")
def test_ingest_schema_matches_openapi_yml():
    call_command("check_contract", contract=str(CONTRACT))  # raises CommandError on any difference
