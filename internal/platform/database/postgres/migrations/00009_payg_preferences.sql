-- +goose Up
ALTER TABLE user_preferences ADD COLUMN payg_enabled BIGINT NOT NULL DEFAULT 1 CHECK(payg_enabled IN (0,1));
CREATE TABLE user_disabled_payg_squads (
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 group_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_groups(id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,group_id)
);
-- +goose StatementBegin
CREATE FUNCTION compile_payg_preference_insert_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF NEW.payg_enabled=0 THEN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_payg_preference_insert AFTER INSERT ON user_preferences FOR EACH ROW EXECUTE FUNCTION compile_payg_preference_insert_fn();
-- +goose StatementBegin
CREATE FUNCTION compile_payg_preference_update_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.payg_enabled<>NEW.payg_enabled THEN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_payg_preference_update AFTER UPDATE OF payg_enabled ON user_preferences FOR EACH ROW EXECUTE FUNCTION compile_payg_preference_update_fn();
-- +goose StatementBegin
CREATE FUNCTION compile_payg_preference_delete_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
IF OLD.payg_enabled=0 THEN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END IF;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_payg_preference_delete AFTER DELETE ON user_preferences FOR EACH ROW EXECUTE FUNCTION compile_payg_preference_delete_fn();
-- +goose StatementBegin
CREATE FUNCTION compile_disabled_payg_insert_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_disabled_payg_insert AFTER INSERT ON user_disabled_payg_squads FOR EACH ROW EXECUTE FUNCTION compile_disabled_payg_insert_fn();
-- +goose StatementBegin
CREATE FUNCTION compile_disabled_payg_delete_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_disabled_payg_delete AFTER DELETE ON user_disabled_payg_squads FOR EACH ROW EXECUTE FUNCTION compile_disabled_payg_delete_fn();
INSERT INTO schema_migrations(version,applied_at) VALUES('097_payg_preferences.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
