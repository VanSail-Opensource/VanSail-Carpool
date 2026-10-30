ALTER TABLE users ADD COLUMN account_status TEXT NOT NULL DEFAULT 'active' CHECK(account_status IN ('active','purging','purged'));
ALTER TABLE users ADD COLUMN account_generation INTEGER NOT NULL DEFAULT 0 CHECK(account_generation>=0);
ALTER TABLE users ADD COLUMN bot_blocked INTEGER NOT NULL DEFAULT 0 CHECK(bot_blocked IN (0,1));
ALTER TABLE users ADD COLUMN bot_event_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN bot_update_id INTEGER NOT NULL DEFAULT -1;
ALTER TABLE users ADD COLUMN last_purged_at TEXT;

-- Gate new authorizations even when a request races the authenticated block event.
CREATE TRIGGER account_purchase_insert BEFORE INSERT ON purchases
WHEN EXISTS(SELECT 1 FROM users WHERE id=NEW.user_id AND account_status<>'active')
BEGIN SELECT RAISE(ABORT,'account is unavailable'); END;
CREATE TRIGGER account_payment_insert BEFORE INSERT ON payment_orders
WHEN EXISTS(SELECT 1 FROM users WHERE id=NEW.user_id AND account_status<>'active')
BEGIN SELECT RAISE(ABORT,'account is unavailable'); END;
CREATE TRIGGER account_session_insert BEFORE INSERT ON sessions
WHEN EXISTS(SELECT 1 FROM users WHERE id=NEW.user_id AND (account_status<>'active' OR bot_blocked=1))
BEGIN SELECT RAISE(ABORT,'account is unavailable'); END;
CREATE TRIGGER account_onboarding_update BEFORE UPDATE OF onboarding_state ON users
WHEN NEW.onboarding_state='complete' AND NEW.account_status<>'active'
BEGIN SELECT RAISE(ABORT,'account is unavailable'); END;
