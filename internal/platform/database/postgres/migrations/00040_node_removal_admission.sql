-- +goose Up
-- Persistent admission fence survives an interrupted accounting drain.
ALTER TABLE xnode_nodes ADD COLUMN removing_at TEXT COLLATE "C";
ALTER TABLE xnode_nodes ADD CONSTRAINT xnode_removing_admission CHECK(removing_at IS NULL OR enabled=0);
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION xnode_active_binding_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM id FROM xnode_nodes WHERE id=NEW.node_id AND retired_at IS NULL AND removing_at IS NULL FOR KEY SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'node is removing, retired or absent' USING ERRCODE='23503'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
-- A disabled node still needs a fresh native configuration when removal starts.
DROP TRIGGER compile_xnode_nodes_update ON xnode_nodes;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION compile_xnode_nodes_update_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.config_id IS DISTINCT FROM NEW.config_id OR OLD.settings_cipher IS DISTINCT FROM NEW.settings_cipher OR OLD.enabled IS DISTINCT FROM NEW.enabled OR OLD.removing_at IS DISTINCT FROM NEW.removing_at THEN
  UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER compile_xnode_nodes_update AFTER UPDATE OF config_id,settings_cipher,enabled,removing_at ON xnode_nodes FOR EACH ROW EXECUTE FUNCTION compile_xnode_nodes_update_fn();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
