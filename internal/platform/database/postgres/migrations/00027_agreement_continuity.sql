-- +goose Up
UPDATE users SET onboarding_state='complete'
WHERE onboarding_state='agreement'
  AND (policy_accepted_at IS NOT NULL OR accepted_agreement_revision>0);

INSERT INTO schema_migrations(version,applied_at) VALUES('113_agreement_continuity.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
