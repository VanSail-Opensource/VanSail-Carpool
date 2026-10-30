-- +goose Up
ALTER TABLE users ADD COLUMN account_status TEXT COLLATE "C" NOT NULL DEFAULT 'active' CHECK(account_status IN ('active','purging','purged'));
ALTER TABLE users ADD COLUMN account_generation BIGINT NOT NULL DEFAULT 0 CHECK(account_generation>=0);
ALTER TABLE users ADD COLUMN bot_blocked BIGINT NOT NULL DEFAULT 0 CHECK(bot_blocked IN (0,1));
ALTER TABLE users ADD COLUMN bot_event_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN bot_update_id BIGINT NOT NULL DEFAULT -1;
ALTER TABLE users ADD COLUMN last_purged_at TEXT;


-- +goose StatementBegin
CREATE FUNCTION account_purchase_insert_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF EXISTS(SELECT 1 FROM users WHERE id=NEW.user_id AND account_status<>'active') THEN RAISE EXCEPTION 'account is unavailable'; END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER account_purchase_insert BEFORE INSERT ON purchases FOR EACH ROW EXECUTE FUNCTION account_purchase_insert_fn();

-- +goose StatementBegin
CREATE FUNCTION account_payment_insert_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF EXISTS(SELECT 1 FROM users WHERE id=NEW.user_id AND account_status<>'active') THEN RAISE EXCEPTION 'account is unavailable'; END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER account_payment_insert BEFORE INSERT ON payment_orders FOR EACH ROW EXECUTE FUNCTION account_payment_insert_fn();

-- +goose StatementBegin
CREATE FUNCTION account_session_insert_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF EXISTS(SELECT 1 FROM users WHERE id=NEW.user_id AND (account_status<>'active' OR bot_blocked=1)) THEN RAISE EXCEPTION 'account is unavailable'; END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER account_session_insert BEFORE INSERT ON sessions FOR EACH ROW EXECUTE FUNCTION account_session_insert_fn();

-- +goose StatementBegin
CREATE FUNCTION account_onboarding_update_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF NEW.onboarding_state='complete' AND NEW.account_status<>'active' THEN RAISE EXCEPTION 'account is unavailable'; END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER account_onboarding_update BEFORE UPDATE OF onboarding_state ON users FOR EACH ROW EXECUTE FUNCTION account_onboarding_update_fn();

INSERT INTO schema_migrations(version,applied_at) VALUES('115_account_lifecycle.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
