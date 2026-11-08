-- +goose Up
-- Forward observed history only. Existing daily usage is never expanded into
-- invented minute samples and existing purchases/enrollments do not convert.
CREATE TABLE xnode_usage_minutes (
 owner_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 account_generation BIGINT CHECK(account_generation>=0),
 billing_mode TEXT COLLATE "C" NOT NULL CHECK(billing_mode IN('combo','hourly')),
 reference_id TEXT COLLATE "C" NOT NULL CHECK(reference_id<>''),
 proxy_user_id BIGINT NOT NULL,
 period_id TEXT COLLATE "C" NOT NULL,
 node_id TEXT COLLATE "C" NOT NULL,
 minute TIMESTAMPTZ NOT NULL,
 observed_from TIMESTAMPTZ NOT NULL,
 observed_through TIMESTAMPTZ NOT NULL,
 upload_bytes NUMERIC NOT NULL CHECK(upload_bytes>=0 AND scale(upload_bytes)=0),
 download_bytes NUMERIC NOT NULL CHECK(download_bytes>=0 AND scale(download_bytes)=0),
 PRIMARY KEY(owner_id,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute),
 CHECK(observed_through>=observed_from),
 CHECK(minute=date_trunc('minute',minute AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
 CHECK(billing_mode<>'hourly' OR account_generation IS NOT NULL)
);
CREATE INDEX xnode_usage_window ON xnode_usage_minutes(owner_id,minute,node_id);
CREATE INDEX xnode_usage_purchase ON xnode_usage_minutes(owner_id,billing_mode,reference_id,minute);
CREATE INDEX xnode_usage_node ON xnode_usage_minutes(node_id);

CREATE TABLE xnode_usage_history_state (
 singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK(singleton),
 started_at TIMESTAMPTZ NOT NULL
);
-- The first committed forward report initializes the floor, not migration time.
CREATE TABLE xnode_usage_coverage (
 node_id TEXT COLLATE "C" NOT NULL,
 agent_id TEXT COLLATE "C" NOT NULL,
 stream_id TEXT COLLATE "C" NOT NULL,
 sequence BIGINT NOT NULL CHECK(sequence>=0),
 digest TEXT COLLATE "C" NOT NULL,
 reason TEXT COLLATE "C" NOT NULL,
 observed_at TIMESTAMPTZ NOT NULL,
 dropped_native NUMERIC NOT NULL CHECK(dropped_native>=0 AND scale(dropped_native)=0),
 dropped_hourly NUMERIC NOT NULL CHECK(dropped_hourly>=0 AND scale(dropped_hourly)=0),
 PRIMARY KEY(node_id,agent_id,stream_id,sequence,digest)
);
CREATE INDEX xnode_usage_coverage_window ON xnode_usage_coverage(observed_at,node_id);
CREATE INDEX xnode_usage_coverage_node ON xnode_usage_coverage(node_id);

-- Paid-boundary and status transitions invalidate disposable compiled access.
-- +goose StatementBegin
CREATE FUNCTION hourly_compile_revision_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER hourly_compile_revision AFTER INSERT OR UPDATE OR DELETE ON hourly_enrollments FOR EACH ROW EXECUTE FUNCTION hourly_compile_revision_fn();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
