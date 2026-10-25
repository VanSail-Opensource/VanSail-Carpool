-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION compile_xnode_node_catalog_metadata_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.name IS DISTINCT FROM NEW.name OR OLD.country_code IS DISTINCT FROM NEW.country_code OR OLD.provider_name IS DISTINCT FROM NEW.provider_name OR OLD.multiplier_fp IS DISTINCT FROM NEW.multiplier_fp THEN
 UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_xnode_node_catalog_metadata AFTER UPDATE OF name,country_code,provider_name,multiplier_fp ON xnode_nodes FOR EACH ROW EXECUTE FUNCTION compile_xnode_node_catalog_metadata_fn();
INSERT INTO schema_migrations(version,applied_at) VALUES('099_catalog_topology_epoch.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
