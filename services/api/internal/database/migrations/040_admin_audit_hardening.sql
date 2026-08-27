CREATE INDEX admin_audit_events_search_idx
    ON admin_audit_events (action, resource_type, resource_id, outcome, occurred_at DESC);

REVOKE UPDATE, DELETE, TRUNCATE ON admin_audit_events FROM PUBLIC;

DO $$
DECLARE
    application_role TEXT := current_setting('app.application_role', TRUE);
BEGIN
    IF application_role IS NOT NULL AND application_role <> '' THEN
        EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON admin_audit_events FROM %I', application_role);
    END IF;
END $$;
