-- +goose Up
-- No old blocked flag or purge request is converted into reachability evidence.
ALTER TABLE purchase_history_tombstones ADD COLUMN purchased_term_seconds BIGINT CHECK(purchased_term_seconds>0);

CREATE TABLE account_cleanup_health (
 singleton BIGINT PRIMARY KEY CHECK(singleton=1),
 bot_id BIGINT NOT NULL DEFAULT 0 CHECK(bot_id>=0),
 bot_epoch BIGINT NOT NULL DEFAULT 0 CHECK(bot_epoch>=0),
 verified BIGINT NOT NULL DEFAULT 0 CHECK(verified IN(0,1)),
 verified_at TIMESTAMPTZ,
 window_start TIMESTAMPTZ,
 checked_count BIGINT NOT NULL DEFAULT 0 CHECK(checked_count>=0),
 unhealthy_count BIGINT NOT NULL DEFAULT 0 CHECK(unhealthy_count>=0 AND unhealthy_count<=checked_count),
 paused_until TIMESTAMPTZ,
 CHECK(verified=0 OR bot_id>0 AND bot_epoch>0 AND verified_at IS NOT NULL)
);
INSERT INTO account_cleanup_health(singleton) VALUES(1);

CREATE TABLE account_cleanup_checks (
 user_id TEXT COLLATE "C" PRIMARY KEY REFERENCES users(id),
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 bot_epoch BIGINT NOT NULL CHECK(bot_epoch>0),
 next_probe_at TIMESTAMPTZ NOT NULL,
 private_reachable_at TIMESTAMPTZ,
 health_observed_at TIMESTAMPTZ,
 first_failure_at TIMESTAMPTZ,
 last_failure_at TIMESTAMPTZ,
 failure_count BIGINT NOT NULL DEFAULT 0 CHECK(failure_count BETWEEN 0 AND 3),
 pending_nonce TEXT COLLATE "C",
 pending_started_at TIMESTAMPTZ,
 pending_final BIGINT NOT NULL DEFAULT 0 CHECK(pending_final IN(0,1)),
 admission_evidence_id TEXT COLLATE "C",
 CHECK((pending_nonce IS NULL)=(pending_started_at IS NULL)),
 CHECK(pending_nonce IS NULL OR length(pending_nonce)=36),
 CHECK(pending_nonce IS NOT NULL OR pending_final=0),
 CHECK((failure_count=0 AND first_failure_at IS NULL AND last_failure_at IS NULL) OR
       (failure_count>0 AND first_failure_at IS NOT NULL AND last_failure_at IS NOT NULL AND last_failure_at>=first_failure_at))
);
CREATE INDEX account_cleanup_checks_due ON account_cleanup_checks(next_probe_at,user_id);

CREATE TABLE account_cleanup_evidence (
 nonce TEXT COLLATE "C" PRIMARY KEY CHECK(length(nonce)=36),
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 bot_id BIGINT NOT NULL CHECK(bot_id>0),
 bot_epoch BIGINT NOT NULL CHECK(bot_epoch>0),
 started_at TIMESTAMPTZ NOT NULL,
 completed_at TIMESTAMPTZ NOT NULL CHECK(completed_at>=started_at),
 source TEXT COLLATE "C" NOT NULL CHECK(source IN('probe','final','authenticated','unblocked')),
 action_class TEXT COLLATE "C" NOT NULL CHECK(action_class IN('reachable','blocked','deactivated','unreachable','unknown')),
 chat_class TEXT COLLATE "C" NOT NULL CHECK(chat_class IN('private','deactivated','unreachable','unknown')),
 qualifying BIGINT NOT NULL CHECK(qualifying IN(0,1)),
 purge_generation BIGINT CHECK(purge_generation>0),
 CHECK(purge_generation IS NULL OR source='final' AND qualifying=1)
);
CREATE INDEX account_cleanup_evidence_owner ON account_cleanup_evidence(user_id,completed_at,nonce);
ALTER TABLE account_cleanup_checks ADD CONSTRAINT account_cleanup_admission_evidence_fk
 FOREIGN KEY(admission_evidence_id) REFERENCES account_cleanup_evidence(nonce);

-- +goose StatementBegin
CREATE FUNCTION account_cleanup_evidence_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'account cleanup evidence is immutable';
END $$;
-- +goose StatementEnd
CREATE TRIGGER account_cleanup_evidence_guard BEFORE UPDATE OR DELETE ON account_cleanup_evidence
 FOR EACH ROW EXECUTE FUNCTION account_cleanup_evidence_guard_fn();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
