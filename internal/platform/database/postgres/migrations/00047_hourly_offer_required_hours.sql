-- +goose Up
-- Each offer may own a funding threshold; zero inherits the account-wide
-- policy so existing rows keep their previous activation requirement.
ALTER TABLE hourly_offers ADD COLUMN required_hours BIGINT NOT NULL DEFAULT 0
 CHECK(required_hours>=0 AND required_hours<=2562047);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
