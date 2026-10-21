-- Immutable attribution connects isolated probe credentials to the originating
-- paid period. Results remain in existing provider_operation_items.
CREATE TABLE xnode_metered_probes (
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id),
 pair_key TEXT NOT NULL,
 endpoint_id TEXT NOT NULL,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 inbound_tag TEXT NOT NULL,
 valid_until TEXT NOT NULL,
 revoked_at TEXT,
 flushed_sequence INTEGER,
 UNIQUE(operation_id,pair_key)
);
CREATE INDEX xnode_metered_probes_period ON xnode_metered_probes(period_id,node_id);
CREATE TRIGGER compile_xnode_probe_insert AFTER INSERT ON xnode_metered_probes BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_xnode_probe_revoke AFTER UPDATE OF revoked_at ON xnode_metered_probes BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
