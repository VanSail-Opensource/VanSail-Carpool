-- +goose Up
ALTER TABLE xnode_nodes ADD COLUMN retired_at TEXT COLLATE "C";
-- Lock the parent so a binding insert cannot race a retirement check.
-- +goose StatementBegin
CREATE FUNCTION xnode_active_binding_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM id FROM xnode_nodes WHERE id=NEW.node_id AND retired_at IS NULL FOR KEY SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'node is retired or absent' USING ERRCODE='23503'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER xnode_group_retirement_guard BEFORE INSERT OR UPDATE ON xnode_group_bindings FOR EACH ROW EXECUTE FUNCTION xnode_active_binding_guard_fn();
CREATE TRIGGER xnode_service_retirement_guard BEFORE INSERT OR UPDATE ON xnode_service_bindings FOR EACH ROW EXECUTE FUNCTION xnode_active_binding_guard_fn();
CREATE TRIGGER xnode_endpoint_retirement_guard BEFORE INSERT OR UPDATE ON xnode_endpoints FOR EACH ROW EXECUTE FUNCTION xnode_active_binding_guard_fn();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
