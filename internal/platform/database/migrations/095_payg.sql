-- PAYG uses separate native principals; existing combo terms remain unchanged.
ALTER TABLE squad_product_overrides ADD COLUMN billing_mode TEXT NOT NULL DEFAULT 'combo' CHECK(billing_mode IN ('combo','payg'));
ALTER TABLE squad_product_overrides ADD COLUMN payg_price_minor INTEGER NOT NULL DEFAULT 0 CHECK(payg_price_minor>=0 AND (billing_mode<>'payg' OR payg_price_minor>0));
ALTER TABLE purchases ADD COLUMN combo_squad_uuids TEXT;
CREATE TABLE xnode_payg_accounts (
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 user_id TEXT NOT NULL REFERENCES users(id), group_id TEXT NOT NULL,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 period_id TEXT NOT NULL UNIQUE REFERENCES xnode_periods(id),
 unit_price_minor INTEGER NOT NULL CHECK(unit_price_minor>0),
 purchased_bytes INTEGER NOT NULL DEFAULT 0 CHECK(purchased_bytes>=0),
 UNIQUE(user_id,group_id,node_id)
);
CREATE INDEX xnode_payg_node ON xnode_payg_accounts(node_id,user_id);
CREATE TABLE xnode_payg_charges (
 ledger_id TEXT PRIMARY KEY REFERENCES ledger_entries(id),
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_payg_accounts(proxy_user_id),
 first_block INTEGER NOT NULL CHECK(first_block>=0),
 blocks INTEGER NOT NULL CHECK(blocks>0),
 unit_price_minor INTEGER NOT NULL CHECK(unit_price_minor>0),
 UNIQUE(proxy_user_id,first_block)
);
CREATE TABLE xnode_payg_minutes (
 user_id TEXT NOT NULL REFERENCES users(id), group_id TEXT NOT NULL,
 minute TEXT NOT NULL, traffic_bytes INTEGER NOT NULL DEFAULT 0 CHECK(traffic_bytes>=0),
 spent_minor INTEGER NOT NULL DEFAULT 0 CHECK(spent_minor>=0),
 PRIMARY KEY(user_id,group_id,minute)
);
CREATE INDEX xnode_payg_minutes_expiry ON xnode_payg_minutes(minute);
CREATE TRIGGER compile_payg_product_insert AFTER INSERT ON squad_product_overrides BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_payg_product_update AFTER UPDATE ON squad_product_overrides BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_payg_product_delete AFTER DELETE ON squad_product_overrides BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_payg_account_insert AFTER INSERT ON xnode_payg_accounts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
