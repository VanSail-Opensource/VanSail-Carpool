-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION compile_xnode_endpoints_epoch_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF ROW(OLD.node_id,OLD.inbound_tag,OLD.name,OLD.address,OLD.port,OLD.enabled,OLD.hidden,OLD.overrides_cipher) IS NOT DISTINCT FROM ROW(NEW.node_id,NEW.inbound_tag,NEW.name,NEW.address,NEW.port,NEW.enabled,NEW.hidden,NEW.overrides_cipher) THEN RETURN NEW; END IF;
 END IF;
 UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_xnode_endpoints_insert AFTER INSERT ON xnode_endpoints FOR EACH ROW EXECUTE FUNCTION compile_xnode_endpoints_epoch_fn();
CREATE TRIGGER compile_xnode_endpoints_delete AFTER DELETE ON xnode_endpoints FOR EACH ROW EXECUTE FUNCTION compile_xnode_endpoints_epoch_fn();
CREATE TRIGGER compile_xnode_endpoints_update AFTER UPDATE OF node_id,inbound_tag,name,address,port,enabled,hidden,overrides_cipher ON xnode_endpoints FOR EACH ROW EXECUTE FUNCTION compile_xnode_endpoints_epoch_fn();
INSERT INTO schema_migrations(version,applied_at) VALUES('102_xnode_endpoint_epoch.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
