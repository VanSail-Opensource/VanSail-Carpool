-- +goose Up
-- New empty accounting relations only; no existing benefit or purchase converts.
CREATE TABLE hourly_credit_days (
 owner_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 day_utc DATE NOT NULL,
 free_allowance_bytes BIGINT NOT NULL CHECK(free_allowance_bytes>=0),
 free_admitted_bytes BIGINT NOT NULL DEFAULT 0 CHECK(free_admitted_bytes>=0),
 free_reserved_bytes BIGINT NOT NULL DEFAULT 0 CHECK(free_reserved_bytes>=0),
 extra_funded_bytes BIGINT NOT NULL DEFAULT 0 CHECK(extra_funded_bytes>=0),
 extra_admitted_bytes BIGINT NOT NULL DEFAULT 0 CHECK(extra_admitted_bytes>=0),
 extra_reserved_bytes BIGINT NOT NULL DEFAULT 0 CHECK(extra_reserved_bytes>=0),
 created_at TEXT COLLATE "C" NOT NULL,
 updated_at TEXT COLLATE "C" NOT NULL,
 PRIMARY KEY(owner_id,account_generation,day_utc),
 CHECK(free_admitted_bytes<=free_allowance_bytes AND free_reserved_bytes<=free_allowance_bytes-free_admitted_bytes),
 CHECK(extra_admitted_bytes<=extra_funded_bytes AND extra_reserved_bytes<=extra_funded_bytes-extra_admitted_bytes)
);

CREATE TABLE hourly_credit_agent_sequences (
 agent_id TEXT COLLATE "C" PRIMARY KEY,
 last_issued BIGINT NOT NULL DEFAULT 0 CHECK(last_issued>=0),
 updated_at TEXT COLLATE "C" NOT NULL
);

CREATE TABLE hourly_credit_reservations (
 reservation_id TEXT COLLATE "C" PRIMARY KEY,
 owner_id TEXT COLLATE "C" NOT NULL,
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 day_utc DATE NOT NULL,
 node_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_nodes(id),
 agent_id TEXT COLLATE "C" NOT NULL REFERENCES hourly_credit_agent_sequences(agent_id),
 boot_id TEXT COLLATE "C" NOT NULL,
 session_id TEXT COLLATE "C" NOT NULL,
 kind TEXT COLLATE "C" NOT NULL CHECK(kind IN ('free','extra')),
 byte_ceiling BIGINT NOT NULL CHECK(byte_ceiling>0),
 issue_sequence BIGINT NOT NULL CHECK(issue_sequence>0),
 admitted_upload_bytes BIGINT NOT NULL DEFAULT 0 CHECK(admitted_upload_bytes>=0),
 admitted_download_bytes BIGINT NOT NULL DEFAULT 0 CHECK(admitted_download_bytes>=0),
 unused_bytes BIGINT NOT NULL DEFAULT 0 CHECK(unused_bytes>=0),
 unknown_bytes BIGINT NOT NULL DEFAULT 0 CHECK(unknown_bytes>=0),
 final_proof_digest TEXT COLLATE "C" NOT NULL DEFAULT '',
 final_recovery BOOLEAN NOT NULL DEFAULT false,
 created_at TEXT COLLATE "C" NOT NULL,
 finalized_at TEXT COLLATE "C",
 FOREIGN KEY(owner_id,account_generation,day_utc) REFERENCES hourly_credit_days(owner_id,account_generation,day_utc),
 CONSTRAINT hourly_credit_agent_issue UNIQUE(agent_id,issue_sequence),
 CHECK(admitted_upload_bytes<=byte_ceiling AND admitted_download_bytes<=byte_ceiling-admitted_upload_bytes),
 CHECK(unused_bytes<=byte_ceiling-admitted_upload_bytes-admitted_download_bytes),
 CHECK(unknown_bytes<=byte_ceiling-admitted_upload_bytes-admitted_download_bytes-unused_bytes),
 CHECK((final_proof_digest='' AND finalized_at IS NULL AND NOT final_recovery AND unused_bytes=0 AND unknown_bytes=0)
  OR (final_proof_digest ~ '^[0-9a-f]{64}$' AND finalized_at IS NOT NULL AND
   ((final_recovery AND unused_bytes=0 AND unknown_bytes=byte_ceiling-admitted_upload_bytes-admitted_download_bytes)
    OR (NOT final_recovery AND unknown_bytes=0 AND unused_bytes=byte_ceiling-admitted_upload_bytes-admitted_download_bytes))))
);
CREATE INDEX hourly_credit_owner_day ON hourly_credit_reservations(owner_id,account_generation,day_utc,reservation_id);
CREATE INDEX hourly_credit_unfinalized_agent ON hourly_credit_reservations(agent_id,issue_sequence) WHERE final_proof_digest='';
CREATE INDEX hourly_credit_outstanding_node ON hourly_credit_reservations(node_id,agent_id,issue_sequence) WHERE final_proof_digest='' OR final_recovery AND unknown_bytes>0;
CREATE INDEX hourly_credit_outstanding_owner ON hourly_credit_reservations(owner_id,account_generation,node_id,agent_id,issue_sequence) WHERE final_proof_digest='' OR final_recovery AND unknown_bytes>0;

-- +goose StatementBegin
CREATE FUNCTION hourly_credit_reservation_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'hourly credit evidence is retained'; END IF;
 IF ROW(NEW.reservation_id,NEW.owner_id,NEW.account_generation,NEW.day_utc,NEW.node_id,NEW.agent_id,NEW.boot_id,NEW.session_id,NEW.kind,NEW.byte_ceiling,NEW.issue_sequence,NEW.created_at)
  IS DISTINCT FROM ROW(OLD.reservation_id,OLD.owner_id,OLD.account_generation,OLD.day_utc,OLD.node_id,OLD.agent_id,OLD.boot_id,OLD.session_id,OLD.kind,OLD.byte_ceiling,OLD.issue_sequence,OLD.created_at)
  OR NEW.admitted_upload_bytes<OLD.admitted_upload_bytes OR NEW.admitted_download_bytes<OLD.admitted_download_bytes
 THEN RAISE EXCEPTION 'hourly credit scope or confirmed usage is immutable'; END IF;
 IF OLD.final_proof_digest<>'' AND
  (ROW(NEW.final_proof_digest,NEW.final_recovery,NEW.unused_bytes,NEW.finalized_at) IS DISTINCT FROM ROW(OLD.final_proof_digest,OLD.final_recovery,OLD.unused_bytes,OLD.finalized_at)
   OR (NOT OLD.final_recovery AND ROW(NEW.admitted_upload_bytes,NEW.admitted_download_bytes) IS DISTINCT FROM ROW(OLD.admitted_upload_bytes,OLD.admitted_download_bytes)))
 THEN RAISE EXCEPTION 'hourly final proof is immutable'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER hourly_credit_reservation_guard BEFORE UPDATE OR DELETE ON hourly_credit_reservations FOR EACH ROW EXECUTE FUNCTION hourly_credit_reservation_guard_fn();

-- +goose StatementBegin
CREATE FUNCTION hourly_credit_day_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.owner_id,NEW.account_generation,NEW.day_utc,NEW.free_allowance_bytes,NEW.created_at) IS DISTINCT FROM ROW(OLD.owner_id,OLD.account_generation,OLD.day_utc,OLD.free_allowance_bytes,OLD.created_at)
  OR NEW.free_admitted_bytes<OLD.free_admitted_bytes OR NEW.extra_admitted_bytes<OLD.extra_admitted_bytes OR NEW.extra_funded_bytes<OLD.extra_funded_bytes
 THEN RAISE EXCEPTION 'hourly daily policy and admitted/funded counters are immutable'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER hourly_credit_day_guard BEFORE UPDATE ON hourly_credit_days FOR EACH ROW EXECUTE FUNCTION hourly_credit_day_guard_fn();

-- +goose StatementBegin
CREATE FUNCTION hourly_credit_sequence_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'hourly agent issue highwater is retained'; END IF;
 IF NEW.agent_id IS DISTINCT FROM OLD.agent_id OR NEW.last_issued<OLD.last_issued THEN RAISE EXCEPTION 'hourly agent issue highwater cannot regress'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER hourly_credit_sequence_guard BEFORE UPDATE OR DELETE ON hourly_credit_agent_sequences FOR EACH ROW EXECUTE FUNCTION hourly_credit_sequence_guard_fn();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
