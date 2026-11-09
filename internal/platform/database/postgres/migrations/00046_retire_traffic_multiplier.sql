-- +goose Up
-- Goose executes this before any query in this migration transaction. A fresh
-- statement snapshot after the table-lock wait must include drained writers.
SET TRANSACTION ISOLATION LEVEL READ COMMITTED;
-- +goose StatementBegin
DO $$ BEGIN
 IF NOT pg_try_advisory_xact_lock(72173680) THEN
  RAISE EXCEPTION 'Stop the active VanSail runtime before retiring traffic multipliers';
 END IF;
END $$;
-- +goose StatementEnd

-- Hold the runtime-owner fence through commit, and drain complete ingest/rate
-- transactions before reading their final committed sequence boundaries.
LOCK TABLE xnode_nodes,xnode_report_cursors,xnode_traffic_streams,
 xnode_node_rates,xnode_stream_rates IN ACCESS EXCLUSIVE MODE;

-- Node FOR UPDATE fences old binding/rate/stream-registration snapshots. Traffic
-- uses compatible node KEY SHARE, so version cursor rows too: its subsequent
-- cursor FOR UPDATE (or missing-cursor INSERT conflict) must reject stale state.
UPDATE xnode_nodes SET multiplier_fp=1000000;
INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at)
 SELECT agent_id,id,0,'',to_char(clock_timestamp() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
 FROM xnode_nodes WHERE agent_id IS NOT NULL AND agent_id<>''
 ON CONFLICT(agent_id) DO NOTHING;
UPDATE xnode_report_cursors SET last_sequence=last_sequence;
UPDATE xnode_traffic_streams SET last_sequence=last_sequence;

-- Only uncommitted policy is normalized. Historical rows and unknown orphan
-- history remain byte-exact. Exhausted sequence domains have no next boundary.
UPDATE xnode_node_rates r SET multiplier_fp=1000000
 FROM xnode_report_cursors c
 WHERE r.agent_id=c.agent_id AND r.from_sequence>c.last_sequence;
INSERT INTO xnode_node_rates(agent_id,from_sequence,multiplier_fp)
 SELECT agent_id,last_sequence+1,1000000 FROM xnode_report_cursors
 WHERE last_sequence<9223372036854775807
 ON CONFLICT(agent_id,from_sequence) DO UPDATE SET multiplier_fp=excluded.multiplier_fp;
UPDATE xnode_stream_rates r SET multiplier_fp=1000000
 FROM xnode_traffic_streams c
 WHERE r.agent_id=c.agent_id AND r.stream_id=c.stream_id AND r.from_sequence>c.last_sequence;
INSERT INTO xnode_stream_rates(agent_id,stream_id,from_sequence,multiplier_fp)
 SELECT agent_id,stream_id,last_sequence+1,1000000 FROM xnode_traffic_streams
 WHERE last_sequence<9223372036854775807
 ON CONFLICT(agent_id,stream_id,from_sequence) DO UPDATE SET multiplier_fp=excluded.multiplier_fp;

-- Old processes may have queued work after losing their owner connection.
-- Their future policy writes cannot restore retired accounting behavior.
-- +goose StatementBegin
CREATE FUNCTION xnode_multiplier_retired_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.multiplier_fp:=1000000;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER xnode_multiplier_retired BEFORE INSERT OR UPDATE OF multiplier_fp ON xnode_nodes
 FOR EACH ROW EXECUTE FUNCTION xnode_multiplier_retired_fn();

-- +goose StatementBegin
CREATE FUNCTION xnode_legacy_rate_retired_fn() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE committed BIGINT;
BEGIN
 SELECT last_sequence INTO committed FROM xnode_report_cursors WHERE agent_id=NEW.agent_id;
 IF TG_OP='UPDATE' THEN
  IF NEW.agent_id IS DISTINCT FROM OLD.agent_id OR NEW.from_sequence<>OLD.from_sequence THEN
   RAISE EXCEPTION 'Traffic rate identity is immutable';
  END IF;
  IF committed IS NULL OR OLD.from_sequence<=committed THEN
   IF NEW.multiplier_fp<>OLD.multiplier_fp THEN
    RAISE EXCEPTION 'Committed traffic rate evidence is immutable';
   END IF;
   RETURN OLD;
  END IF;
 ELSIF NEW.from_sequence<=COALESCE(committed,0) THEN
  RETURN NULL; -- A repeated legacy bind must not insert or replace past policy.
 END IF;
 NEW.multiplier_fp:=1000000;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER xnode_legacy_rate_retired BEFORE INSERT OR UPDATE ON xnode_node_rates
 FOR EACH ROW EXECUTE FUNCTION xnode_legacy_rate_retired_fn();

-- +goose StatementBegin
CREATE FUNCTION xnode_stream_rate_retired_fn() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE committed BIGINT;
BEGIN
 SELECT last_sequence INTO committed FROM xnode_traffic_streams
  WHERE agent_id=NEW.agent_id AND stream_id=NEW.stream_id;
 IF TG_OP='UPDATE' THEN
  IF NEW.agent_id IS DISTINCT FROM OLD.agent_id OR NEW.stream_id IS DISTINCT FROM OLD.stream_id
   OR NEW.from_sequence<>OLD.from_sequence THEN
   RAISE EXCEPTION 'Traffic rate identity is immutable';
  END IF;
  IF committed IS NULL OR OLD.from_sequence<=committed THEN
   IF NEW.multiplier_fp<>OLD.multiplier_fp THEN
    RAISE EXCEPTION 'Committed traffic rate evidence is immutable';
   END IF;
   RETURN OLD;
  END IF;
 ELSIF NEW.from_sequence<=COALESCE(committed,0) THEN
  RETURN NULL;
 END IF;
 NEW.multiplier_fp:=1000000;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER xnode_stream_rate_retired BEFORE INSERT OR UPDATE ON xnode_stream_rates
 FOR EACH ROW EXECUTE FUNCTION xnode_stream_rate_retired_fn();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
