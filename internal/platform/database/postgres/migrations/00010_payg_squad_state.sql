-- +goose Up
ALTER TABLE squad_product_overrides ADD COLUMN payg_state TEXT COLLATE "C" NOT NULL DEFAULT 'normal'
 CHECK(payg_state IN ('normal','hidden','unavailable'));
INSERT INTO schema_migrations(version,applied_at) VALUES('098_payg_squad_state.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
