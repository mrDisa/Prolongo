import logging
import signal
import time

from django.core.management.base import BaseCommand
from django.db import close_old_connections

from source.services.simulator import get_state, run_tick

logger = logging.getLogger(__name__)


class Command(BaseCommand):
    help = "Simulator loop: every second reads simulator_state and runs a tick when it is due."

    def handle(self, *args, **options):
        stopping = False

        def stop(signum, frame):
            nonlocal stopping
            stopping = True

        # The process is PID 1 in its container: without handlers SIGTERM would be ignored.
        signal.signal(signal.SIGTERM, stop)
        signal.signal(signal.SIGINT, stop)

        self.stdout.write("Simulator loop started.")
        last_tick = None
        while not stopping:
            close_old_connections()  # survive database restarts
            try:
                state = get_state()
                if not state.running:
                    last_tick = None
                elif last_tick is None or time.monotonic() - last_tick >= state.interval_seconds:
                    last_tick = time.monotonic()
                    result = run_tick()
                    self.stdout.write(
                        f"tick: dataTime={result['dataTime'].isoformat()} "
                        f"statusChanges={result['statusChanges']} randomEvents={result['randomEvents']}"
                    )
            except Exception:
                logger.exception("Simulator tick failed")
            time.sleep(1)
        self.stdout.write("Simulator loop stopped.")
