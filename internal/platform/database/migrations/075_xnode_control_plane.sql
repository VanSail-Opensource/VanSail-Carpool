-- The panel becomes authoritative for node configuration, access, and accounting.
-- Live connections, source-IP snapshots, rates, and admission decisions stay in memory.
CREATE TABLE xnode_configs (
 id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, document_cipher TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE xnode_snippets (
 id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, kind TEXT NOT NULL CHECK(kind IN ('outbound','routing')),
 document_cipher TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE xnode_config_snippets (
 config_id TEXT NOT NULL REFERENCES xnode_configs(id) ON DELETE CASCADE,
 snippet_id TEXT NOT NULL REFERENCES xnode_snippets(id), position INTEGER NOT NULL,
 PRIMARY KEY(config_id,snippet_id), UNIQUE(config_id,position)
);
CREATE TABLE xnode_nodes (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, country_code TEXT NOT NULL DEFAULT '',
 provider_name TEXT NOT NULL DEFAULT '', multiplier_fp INTEGER NOT NULL DEFAULT 1000000 CHECK(multiplier_fp>0),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
 config_id TEXT NOT NULL REFERENCES xnode_configs(id), settings_cipher TEXT NOT NULL,
 token_hash BLOB UNIQUE CHECK(token_hash IS NULL OR length(token_hash)=32), agent_id TEXT UNIQUE,
 desired_revision INTEGER NOT NULL DEFAULT 0, accepted_revision INTEGER NOT NULL DEFAULT 0,
 applied_revision INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE xnode_groups (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)),
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE xnode_group_bindings (
 group_id TEXT NOT NULL REFERENCES xnode_groups(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 PRIMARY KEY(group_id,node_id,inbound_tag)
);
CREATE TABLE xnode_endpoints (
 id TEXT PRIMARY KEY, node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 name TEXT NOT NULL, address TEXT NOT NULL, port INTEGER NOT NULL CHECK(port BETWEEN 1 AND 65535),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN(0,1)), hidden INTEGER NOT NULL DEFAULT 0 CHECK(hidden IN(0,1)),
 overrides_cipher TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX xnode_endpoints_node_idx ON xnode_endpoints(node_id,inbound_tag);
CREATE TABLE xnode_identities (
 proxy_user_id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id TEXT UNIQUE REFERENCES users(id), service_name TEXT UNIQUE,
 credentials_cipher TEXT NOT NULL, subscription_hash BLOB UNIQUE,
 subscription_cipher TEXT NOT NULL DEFAULT '', current_period_id TEXT,
 legacy_user_id TEXT, lifetime_base INTEGER NOT NULL DEFAULT 0 CHECK(lifetime_base>=0),
 access_disabled INTEGER NOT NULL DEFAULT 0 CHECK(access_disabled IN(0,1)), created_at TEXT NOT NULL,
 CHECK((user_id IS NOT NULL AND service_name IS NULL) OR (user_id IS NULL AND service_name IS NOT NULL))
);
CREATE TABLE xnode_periods (
 id TEXT PRIMARY KEY, proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 purchase_id TEXT REFERENCES purchases(id), started_at TEXT NOT NULL, ended_at TEXT,
 limit_bytes INTEGER NOT NULL CHECK(limit_bytes>=0), strategy TEXT NOT NULL,
 last_reset_at TEXT, used_bytes INTEGER NOT NULL DEFAULT 0 CHECK(used_bytes>=0),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN('open','closing','closed','review'))
);
CREATE TABLE xnode_service_bindings (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), inbound_tag TEXT NOT NULL,
 valid_until TEXT NOT NULL, PRIMARY KEY(proxy_user_id,node_id,inbound_tag)
);
CREATE INDEX xnode_periods_user_idx ON xnode_periods(proxy_user_id,started_at);
-- Admission participants make settlement conservative when a node is unreachable.
CREATE TABLE xnode_period_nodes (
 period_id TEXT NOT NULL REFERENCES xnode_periods(id), node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 flushed_sequence INTEGER, PRIMARY KEY(period_id,node_id)
);
CREATE TABLE xnode_daily_usage (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 date TEXT NOT NULL, upload_bytes INTEGER NOT NULL DEFAULT 0 CHECK(upload_bytes>=0),
 download_bytes INTEGER NOT NULL DEFAULT 0 CHECK(download_bytes>=0),
 weighted_bytes INTEGER NOT NULL DEFAULT 0 CHECK(weighted_bytes>=0),
 weight_remainder INTEGER NOT NULL DEFAULT 0 CHECK(weight_remainder BETWEEN 0 AND 999999),
 PRIMARY KEY(proxy_user_id,node_id,period_id,date)
);
CREATE TABLE xnode_report_cursors (
 agent_id TEXT PRIMARY KEY, node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 last_sequence INTEGER NOT NULL DEFAULT 0, last_digest TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);
-- Source daily history never changes native quota counters.
CREATE TABLE xnode_legacy_history (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 node_id TEXT NOT NULL, date TEXT NOT NULL,
 raw_bytes INTEGER NOT NULL CHECK(raw_bytes>=0), weighted_bytes INTEGER NOT NULL CHECK(weighted_bytes>=0),
 PRIMARY KEY(proxy_user_id,node_id,date)
);
CREATE TABLE xnode_legacy_usage (
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT REFERENCES xnode_periods(id), used_bytes INTEGER NOT NULL CHECK(used_bytes>=0),
 lifetime_bytes INTEGER NOT NULL CHECK(lifetime_bytes>=0), last_reset_at TEXT, synced_at TEXT NOT NULL
);
CREATE TABLE xnode_node_rates (
 agent_id TEXT NOT NULL, from_sequence INTEGER NOT NULL,
 multiplier_fp INTEGER NOT NULL CHECK(multiplier_fp>0), PRIMARY KEY(agent_id,from_sequence)
);
CREATE TABLE xnode_customizations (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), outbounds_cipher TEXT NOT NULL, routing_cipher TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1, updated_at TEXT NOT NULL, PRIMARY KEY(user_id,node_id)
);
CREATE TABLE xnode_migrations (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), phase TEXT NOT NULL,
 source_cipher TEXT NOT NULL, snapshot_cipher TEXT NOT NULL DEFAULT '', snapshot_digest TEXT NOT NULL DEFAULT '',
 source_backup_reference TEXT NOT NULL DEFAULT '', backup_name TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);
CREATE TABLE xnode_migration_nodes (
 node_id TEXT PRIMARY KEY REFERENCES xnode_nodes(id), legacy_node_id TEXT NOT NULL UNIQUE,
 phase TEXT NOT NULL CHECK(phase IN('legacy','preparing','native','rolling_back','review')),
 source_digest TEXT NOT NULL, usage_cutoff TEXT, error_code TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);
