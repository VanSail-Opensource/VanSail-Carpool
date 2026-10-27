-- +goose Up
-- Restore the pricing command lookup index already required by the source contract.
CREATE INDEX pricing_active_commands ON pricing_admin_overrides(command,status,expires_at);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
