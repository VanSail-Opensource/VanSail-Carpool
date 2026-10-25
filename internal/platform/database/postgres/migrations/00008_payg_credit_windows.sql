-- +goose Up
ALTER TABLE xnode_payg_accounts ADD COLUMN credit_bytes BIGINT NOT NULL DEFAULT 0 CHECK(credit_bytes>=0);
ALTER TABLE xnode_payg_accounts ADD COLUMN credit_started_at TEXT COLLATE "C";
UPDATE xnode_payg_accounts a SET credit_bytes=GREATEST(0,purchased_bytes-(SELECT used_bytes FROM xnode_periods WHERE id=a.period_id)),
 credit_started_at=(SELECT l.created_at FROM xnode_payg_charges c JOIN ledger_entries l ON l.id=c.ledger_id WHERE c.proxy_user_id=a.proxy_user_id ORDER BY c.first_block DESC LIMIT 1);
CREATE TABLE xnode_payg_windows (
 proxy_user_id BIGINT NOT NULL REFERENCES xnode_payg_accounts(proxy_user_id),
 through TEXT COLLATE "C" NOT NULL,
 traffic_bytes BIGINT NOT NULL DEFAULT 0 CHECK(traffic_bytes>=0),
 unbilled_bytes BIGINT NOT NULL DEFAULT 0 CHECK(unbilled_bytes>=0 AND unbilled_bytes<=traffic_bytes),
 PRIMARY KEY(proxy_user_id,through)
);
CREATE INDEX xnode_payg_windows_expiry ON xnode_payg_windows(through);
INSERT INTO schema_migrations(version,applied_at) VALUES('096_payg_credit_windows.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
