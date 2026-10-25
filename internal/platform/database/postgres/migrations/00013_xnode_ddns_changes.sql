-- +goose Up
CREATE TABLE xnode_ddns_changes (
 node_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,
 job_id TEXT COLLATE "C" NOT NULL,agent_id TEXT COLLATE "C" NOT NULL,event_id TEXT COLLATE "C" NOT NULL,
 generation TEXT COLLATE "C" NOT NULL,sequence TEXT COLLATE "C" NOT NULL,
 ipv4 TEXT COLLATE "C" NOT NULL DEFAULT '',ipv6 TEXT COLLATE "C" NOT NULL DEFAULT '',observed_at TEXT COLLATE "C" NOT NULL,
 previous_ips TEXT COLLATE "C" NOT NULL DEFAULT '[]' CHECK(json_valid(previous_ips)),
 operation_id TEXT COLLATE "C" NOT NULL REFERENCES provider_operations(id),
 phase TEXT COLLATE "C" NOT NULL CHECK(phase IN ('staging','routes','settled','obsolete')),
 PRIMARY KEY(node_id,job_id)
);
CREATE INDEX xnode_ddns_changes_operation_idx ON xnode_ddns_changes(operation_id);
INSERT INTO schema_migrations(version,applied_at) VALUES('101_xnode_ddns_changes.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
