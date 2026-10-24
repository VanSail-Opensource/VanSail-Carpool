-- +goose Up
ALTER TABLE outbox_jobs ADD COLUMN claim_generation BIGINT NOT NULL DEFAULT 0 CHECK(claim_generation>=0);
CREATE SEQUENCE xnode_service_proxy_id_seq AS BIGINT START WITH 4503599627370496 MAXVALUE 9007199254740991 OWNED BY xnode_identities.proxy_user_id;
-- +goose StatementBegin
CREATE FUNCTION vansail_outbox_notify() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN PERFORM pg_notify('vansail_outbox',''); RETURN NULL; END $$;
-- +goose StatementEnd
CREATE TRIGGER vansail_outbox_wakeup AFTER INSERT OR UPDATE OF status,available_at ON outbox_jobs FOR EACH STATEMENT EXECUTE FUNCTION vansail_outbox_notify();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
