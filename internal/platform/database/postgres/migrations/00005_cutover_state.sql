-- +goose Up
CREATE TABLE vansail_migration_state(singleton BIGINT PRIMARY KEY CHECK(singleton=1),phase TEXT NOT NULL CHECK(phase IN('fresh','imported','verified','active')),source_hash TEXT NOT NULL DEFAULT '',verification_hash TEXT NOT NULL DEFAULT '',verified_at TEXT NOT NULL DEFAULT '');
INSERT INTO vansail_migration_state(singleton,phase) VALUES(1,'fresh');
-- Import defers all original FKs, including self-referencing Telegram inviters.
-- +goose StatementBegin
DO $$ DECLARE item record; BEGIN
 FOR item IN SELECT c.conname,t.relname FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE c.contype='f' AND n.nspname='public' LOOP
 EXECUTE format('ALTER TABLE %I ALTER CONSTRAINT %I DEFERRABLE INITIALLY IMMEDIATE',item.relname,item.conname);
 END LOOP;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
