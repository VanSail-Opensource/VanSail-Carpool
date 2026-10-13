CREATE TABLE payment_orders_usdt (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id),
 provider TEXT NOT NULL CHECK(provider IN ('ezpay','bepusdt','stars','builtin_usdt')),
 status TEXT NOT NULL CHECK(status IN ('creating','pending','paid','expired','failed','refunded')),
 txb_minor INTEGER NOT NULL CHECK(txb_minor>0),
 payable_amount TEXT NOT NULL, payable_currency TEXT NOT NULL, rate_snapshot TEXT NOT NULL,
 provider_trade_id TEXT UNIQUE, provider_charge_id TEXT UNIQUE,
 payment_url TEXT, qr_payload TEXT, provider_payload TEXT NOT NULL DEFAULT '{}',
 expires_at TEXT NOT NULL, paid_at TEXT, refunded_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 method_id TEXT NOT NULL DEFAULT '', provider_rail TEXT NOT NULL DEFAULT '',
 rate_direction TEXT NOT NULL DEFAULT 'currency_per_txb' CHECK(rate_direction IN ('currency_per_txb','txb_per_currency')),
 receiving_address TEXT, actual_crypto_amount TEXT, actual_crypto_currency TEXT,
 cancelled_at TEXT, cancel_reason TEXT NOT NULL DEFAULT '',
 provider_cancel_status TEXT NOT NULL DEFAULT '' CHECK(provider_cancel_status IN ('','unsupported','requested','cancelled','failed','pending_review'))
);
INSERT INTO payment_orders_usdt SELECT * FROM payment_orders;
DROP TABLE payment_orders;
ALTER TABLE payment_orders_usdt RENAME TO payment_orders;
CREATE INDEX payment_orders_user_created_idx ON payment_orders(user_id,created_at DESC);
CREATE INDEX payment_orders_method_created_idx ON payment_orders(method_id,created_at DESC);
CREATE INDEX payment_orders_admin_page_idx ON payment_orders(created_at DESC,id DESC);
CREATE INDEX payment_orders_admin_status_page_idx ON payment_orders(status,created_at DESC,id DESC);

-- Permanent fingerprints have no order FK so paid-detail cleanup cannot allow reuse.
CREATE TABLE usdt_quotes (
 order_id TEXT PRIMARY KEY, network TEXT NOT NULL, token TEXT NOT NULL,
 recipient TEXT NOT NULL, amount TEXT NOT NULL, atomic_amount TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 UNIQUE(network,token,recipient,atomic_amount)
);
CREATE TABLE usdt_transfers (
 id TEXT PRIMARY KEY, network TEXT NOT NULL, token TEXT NOT NULL, recipient TEXT NOT NULL,
 atomic_amount TEXT NOT NULL, amount TEXT NOT NULL, tx_hash TEXT NOT NULL, event_id TEXT NOT NULL,
 occurred_at TEXT NOT NULL, order_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('ready','review','credited')),
 reason TEXT NOT NULL DEFAULT '', received_at TEXT NOT NULL,
 UNIQUE(network,tx_hash,event_id)
);
CREATE INDEX usdt_transfers_review_idx ON usdt_transfers(status,received_at,id);
CREATE TABLE usdt_scan_cursors (
 network TEXT PRIMARY KEY, value TEXT NOT NULL, hash TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE usdt_rpc_state (
 group_id TEXT PRIMARY KEY, next_at TEXT NOT NULL, cooldown_until TEXT NOT NULL, failures INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE usdt_rpc_usage (
 group_id TEXT NOT NULL, minute TEXT NOT NULL, count INTEGER NOT NULL, PRIMARY KEY(group_id,minute)
);
