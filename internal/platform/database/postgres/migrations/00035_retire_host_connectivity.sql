-- +goose Up
-- Host-check results are disposable diagnostics; all financial and audit evidence remains.
DROP TABLE host_connectivity_attempts;
DELETE FROM settings WHERE key = 'connectivity.config';

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
