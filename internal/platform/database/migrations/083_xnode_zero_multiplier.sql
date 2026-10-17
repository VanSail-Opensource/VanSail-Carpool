-- Permit explicit free-node accounting without rewriting historical rates.
-- The dedicated migration connection retains schema objects and validates FKs.

CREATE TABLE xnode_nodes_nonnegative (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, country_code TEXT NOT NULL DEFAULT '',
 provider_name TEXT NOT NULL DEFAULT '', multiplier_fp INTEGER NOT NULL DEFAULT 1000000 CHECK(multiplier_fp>=0),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
 config_id TEXT NOT NULL REFERENCES xnode_configs(id), settings_cipher TEXT NOT NULL,
 token_hash BLOB UNIQUE CHECK(token_hash IS NULL OR length(token_hash)=32), agent_id TEXT UNIQUE,
 desired_revision INTEGER NOT NULL DEFAULT 0, accepted_revision INTEGER NOT NULL DEFAULT 0,
 applied_revision INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
INSERT INTO xnode_nodes_nonnegative SELECT * FROM xnode_nodes;
DROP TABLE xnode_nodes;
ALTER TABLE xnode_nodes_nonnegative RENAME TO xnode_nodes;

CREATE TABLE xnode_node_rates_nonnegative (
 agent_id TEXT NOT NULL, from_sequence INTEGER NOT NULL,
 multiplier_fp INTEGER NOT NULL CHECK(multiplier_fp>=0), PRIMARY KEY(agent_id,from_sequence)
);
INSERT INTO xnode_node_rates_nonnegative SELECT * FROM xnode_node_rates;
DROP TABLE xnode_node_rates;
ALTER TABLE xnode_node_rates_nonnegative RENAME TO xnode_node_rates;
