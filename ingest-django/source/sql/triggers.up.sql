-- Change capture for the source tables.
-- Session settings (set by services/db.py set_context, transaction-local):
--   prolongo.source      who changed the data (snapshot / event / simulator / admin / ...), default 'sql'
--   prolongo.event_time  when the change happened in source time, default source_clock.data_time
--   prolongo.batch_id    ingest_batches.id of the change, optional

CREATE OR REPLACE FUNCTION public.prolongo_event_time() RETURNS timestamptz
LANGUAGE sql STABLE AS $$
  SELECT COALESCE(
    NULLIF(current_setting('prolongo.event_time', true), '')::timestamptz,
    (SELECT data_time FROM public.source_clock WHERE id = 1),
    now())
$$;

CREATE OR REPLACE FUNCTION public.prolongo_service_status_audit() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND NEW.status IS DISTINCT FROM OLD.status THEN
    INSERT INTO public.status_events (service_id, account_id, from_status, to_status, occurred_at, source, batch_id)
    VALUES (NEW.id, NEW."ownerId", OLD.status, NEW.status, public.prolongo_event_time(),
            COALESCE(NULLIF(current_setting('prolongo.source', true), ''), 'sql'),
            NULLIF(current_setting('prolongo.batch_id', true), '')::bigint);
  ELSIF TG_OP = 'DELETE' AND OLD.status <> 'deleted' THEN
    -- the service vanished from the source without being deleted: treat it as a deletion
    INSERT INTO public.status_events (service_id, account_id, from_status, to_status, occurred_at, source, batch_id)
    VALUES (OLD.id, OLD."ownerId", OLD.status, 'deleted', public.prolongo_event_time(),
            COALESCE(NULLIF(current_setting('prolongo.source', true), ''), 'sql'),
            NULLIF(current_setting('prolongo.batch_id', true), '')::bigint);
  END IF;
  RETURN NULL;
END $$;

CREATE TRIGGER services_status_audit
AFTER UPDATE OF status OR DELETE ON public.services
FOR EACH ROW EXECUTE FUNCTION public.prolongo_service_status_audit();

CREATE OR REPLACE FUNCTION public.prolongo_notify_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('data_changed', '');  -- identical notifications in one transaction are merged by PostgreSQL
  RETURN NULL;
END $$;

CREATE TRIGGER accounts_notify AFTER INSERT OR UPDATE OR DELETE ON public.accounts
FOR EACH STATEMENT EXECUTE FUNCTION public.prolongo_notify_changed();
CREATE TRIGGER services_notify AFTER INSERT OR UPDATE OR DELETE ON public.services
FOR EACH STATEMENT EXECUTE FUNCTION public.prolongo_notify_changed();
CREATE TRIGGER transactions_notify AFTER INSERT OR UPDATE OR DELETE ON public.transactions
FOR EACH STATEMENT EXECUTE FUNCTION public.prolongo_notify_changed();
CREATE TRIGGER source_clock_notify AFTER INSERT OR UPDATE ON public.source_clock
FOR EACH STATEMENT EXECUTE FUNCTION public.prolongo_notify_changed();
