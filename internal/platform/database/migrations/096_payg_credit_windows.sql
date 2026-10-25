-- Remaining prepaid credit is operational state; original charges stay immutable.
ALTER TABLE xnode_payg_accounts ADD COLUMN credit_bytes INTEGER NOT NULL DEFAULT 0 CHECK(credit_bytes>=0);
ALTER TABLE xnode_payg_accounts ADD COLUMN credit_started_at TEXT;
UPDATE xnode_payg_accounts SET credit_bytes=MAX(0,purchased_bytes-(SELECT p.used_bytes FROM xnode_periods p WHERE p.id=xnode_payg_accounts.period_id)),
 credit_started_at=(SELECT l.created_at FROM xnode_payg_charges c JOIN ledger_entries l ON l.id=c.ledger_id WHERE c.proxy_user_id=xnode_payg_accounts.proxy_user_id ORDER BY c.first_block DESC LIMIT 1);
CREATE TABLE xnode_payg_windows (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_payg_accounts(proxy_user_id),
 through TEXT NOT NULL,
 traffic_bytes INTEGER NOT NULL DEFAULT 0 CHECK(traffic_bytes>=0),
 unbilled_bytes INTEGER NOT NULL DEFAULT 0 CHECK(unbilled_bytes>=0 AND unbilled_bytes<=traffic_bytes),
 PRIMARY KEY(proxy_user_id,through)
);
CREATE INDEX xnode_payg_windows_expiry ON xnode_payg_windows(through);
