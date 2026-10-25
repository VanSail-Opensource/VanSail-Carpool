-- Desired address mutation and replay ordering survive reconnects/restarts.
-- No job FK: old attestations can still protect active routes after job removal.
CREATE TABLE xnode_ddns_changes (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,
 job_id TEXT NOT NULL,agent_id TEXT NOT NULL,event_id TEXT NOT NULL,
 generation TEXT NOT NULL,sequence TEXT NOT NULL,
 ipv4 TEXT NOT NULL DEFAULT '',ipv6 TEXT NOT NULL DEFAULT '',observed_at TEXT NOT NULL,
 previous_ips TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(previous_ips)),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id),
 phase TEXT NOT NULL CHECK(phase IN ('staging','routes','settled','obsolete')),
 PRIMARY KEY(node_id,job_id)
);
CREATE INDEX xnode_ddns_changes_operation_idx ON xnode_ddns_changes(operation_id);
