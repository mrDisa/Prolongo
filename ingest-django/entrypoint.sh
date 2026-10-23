#!/bin/sh
set -e
python manage.py migrate --noinput
python manage.py load_dataset /app/data/test-data/data.json --if-empty
if [ "$DEMO_DATA" = "1" ]; then python manage.py generate_demo --accounts 300 --seed 7 --backfill-days 90; fi
python manage.py collectstatic --noinput
exec gunicorn prolongo_ingest.wsgi:application -b 0.0.0.0:8001 -w 3 --timeout 60
