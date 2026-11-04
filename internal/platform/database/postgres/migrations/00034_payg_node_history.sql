-- +goose Up
-- Disposable directional samples only. Financial PAYG windows, charges and
-- historical accounting remain unchanged. Node deletion must not erase history.
CREATE TABLE xnode_payg_node_minutes (
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 node_id TEXT COLLATE "C" NOT NULL,
 minute TIMESTAMPTZ NOT NULL,
 upload_bytes NUMERIC(30,0) NOT NULL DEFAULT 0 CHECK(upload_bytes>=0),
 download_bytes NUMERIC(30,0) NOT NULL DEFAULT 0 CHECK(download_bytes>=0),
 PRIMARY KEY(user_id,node_id,minute)
);
CREATE INDEX xnode_payg_node_minutes_expiry ON xnode_payg_node_minutes(minute);
CREATE INDEX xnode_payg_node_minutes_user_time ON xnode_payg_node_minutes(user_id,minute,node_id);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
