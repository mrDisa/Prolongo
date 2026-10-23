DROP TRIGGER IF EXISTS source_clock_notify ON public.source_clock;
DROP TRIGGER IF EXISTS transactions_notify ON public.transactions;
DROP TRIGGER IF EXISTS services_notify ON public.services;
DROP TRIGGER IF EXISTS accounts_notify ON public.accounts;
DROP TRIGGER IF EXISTS services_status_audit ON public.services;

DROP FUNCTION IF EXISTS public.prolongo_notify_changed();
DROP FUNCTION IF EXISTS public.prolongo_service_status_audit();
DROP FUNCTION IF EXISTS public.prolongo_event_time();
