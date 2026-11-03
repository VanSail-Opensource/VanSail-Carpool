-- +goose Up
-- Only deduplication, rate boundaries and settlement evidence are durable.
-- No traffic buffers or live IP observations enter these relations.
CREATE TABLE xnode_traffic_streams (
 agent_id TEXT COLLATE "C" NOT NULL,
 stream_id TEXT COLLATE "C" NOT NULL,
 node_id TEXT COLLATE "C" NOT NULL,
 last_sequence BIGINT NOT NULL DEFAULT 0 CHECK(last_sequence>=0),
 last_digest TEXT COLLATE "C" NOT NULL DEFAULT '',
 closed BOOLEAN NOT NULL DEFAULT false,
 has_gaps BOOLEAN NOT NULL DEFAULT false,
 created_at TEXT COLLATE "C" NOT NULL,
 updated_at TEXT COLLATE "C" NOT NULL,
 PRIMARY KEY(agent_id,stream_id)
);
CREATE UNIQUE INDEX xnode_traffic_active_stream ON xnode_traffic_streams(agent_id) WHERE NOT closed;

CREATE TABLE xnode_stream_rates (
 agent_id TEXT COLLATE "C" NOT NULL,
 stream_id TEXT COLLATE "C" NOT NULL,
 from_sequence BIGINT NOT NULL CHECK(from_sequence>0),
 multiplier_fp BIGINT NOT NULL CHECK(multiplier_fp>=0),
 PRIMARY KEY(agent_id,stream_id,from_sequence),
 FOREIGN KEY(agent_id,stream_id) REFERENCES xnode_traffic_streams(agent_id,stream_id)
);
ALTER TABLE xnode_period_nodes ADD COLUMN flushed_stream_id TEXT COLLATE "C" NOT NULL DEFAULT '';
ALTER TABLE xnode_period_nodes ADD COLUMN flushed_legacy_sequence BIGINT NOT NULL DEFAULT 0 CHECK(flushed_legacy_sequence>=0);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
