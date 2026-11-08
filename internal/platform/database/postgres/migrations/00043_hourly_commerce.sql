-- +goose Up
-- New offers and activation scopes only. No existing purchase converts.
ALTER TABLE purchases ADD COLUMN purchased_term_seconds BIGINT CHECK(purchased_term_seconds>0);

CREATE TABLE hourly_offers (
 group_id TEXT COLLATE "C" PRIMARY KEY REFERENCES xnode_groups(id) ON DELETE CASCADE,
 enabled BIGINT NOT NULL DEFAULT 0 CHECK(enabled IN(0,1)),
 hourly_price_minor BIGINT NOT NULL CHECK(hourly_price_minor>=0),
 revision BIGINT NOT NULL CHECK(revision>0),
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 CHECK(enabled=0 OR hourly_price_minor>0)
);

CREATE TABLE hourly_enrollments (
 id TEXT COLLATE "C" PRIMARY KEY,
 owner_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 group_id TEXT COLLATE "C" NOT NULL CHECK(group_id<>''),
 activation_key TEXT COLLATE "C" NOT NULL CHECK(activation_key<>''),
 request_fingerprint TEXT COLLATE "C" NOT NULL CHECK(request_fingerprint ~ '^[0-9a-f]{64}$'),
 status TEXT COLLATE "C" NOT NULL CHECK(status IN('preparing','active','funding_suspended','disabled','closed')),
 activated_at TIMESTAMPTZ,
 paid_through TIMESTAMPTZ,
 disabled_at TIMESTAMPTZ,
 cooldown_until TIMESTAMPTZ,
 proxy_user_id BIGINT REFERENCES xnode_identities(proxy_user_id),
 period_id TEXT COLLATE "C" REFERENCES xnode_periods(id),
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 CONSTRAINT hourly_enrollments_activation_replay UNIQUE(owner_id,account_generation,activation_key),
 CONSTRAINT hourly_enrollments_principal UNIQUE(proxy_user_id),
 CONSTRAINT hourly_enrollments_period UNIQUE(period_id),
 CHECK((proxy_user_id IS NULL)=(period_id IS NULL)),
 CHECK((activated_at IS NULL)=(paid_through IS NULL)),
 CHECK(paid_through IS NULL OR paid_through>activated_at),
 CHECK(status NOT IN('active','funding_suspended') OR (activated_at IS NOT NULL AND proxy_user_id IS NOT NULL)),
 CHECK(cooldown_until IS NULL OR (disabled_at IS NOT NULL AND cooldown_until>=disabled_at))
);
CREATE UNIQUE INDEX hourly_enrollments_open_squad ON hourly_enrollments(owner_id,account_generation,group_id) WHERE status IN('preparing','active');
CREATE INDEX hourly_enrollments_due ON hourly_enrollments(paid_through,owner_id) WHERE status='active';
CREATE INDEX hourly_enrollments_owner ON hourly_enrollments(owner_id,account_generation,activated_at,group_id,id);
CREATE INDEX hourly_enrollments_cooldown ON hourly_enrollments(owner_id,account_generation,group_id,cooldown_until DESC) WHERE cooldown_until IS NOT NULL;

CREATE TABLE hourly_settlements (
 id TEXT COLLATE "C" PRIMARY KEY,
 enrollment_id TEXT COLLATE "C" NOT NULL REFERENCES hourly_enrollments(id),
 hour_start TIMESTAMPTZ NOT NULL,
 charged_from TIMESTAMPTZ NOT NULL,
 paid_until TIMESTAMPTZ NOT NULL,
 hourly_price_minor BIGINT NOT NULL CHECK(hourly_price_minor>0),
 charged_minor BIGINT NOT NULL CHECK(charged_minor>0 AND charged_minor<=hourly_price_minor),
 offer_revision BIGINT NOT NULL CHECK(offer_revision>0),
 policy_version TEXT COLLATE "C" NOT NULL CHECK(policy_version ~ '^[0-9a-f]{64}$'),
 ledger_id TEXT COLLATE "C" NOT NULL REFERENCES ledger_entries(id),
 created_at TIMESTAMPTZ NOT NULL,
 CONSTRAINT hourly_settlements_hour UNIQUE(enrollment_id,hour_start),
 CONSTRAINT hourly_settlements_ledger UNIQUE(ledger_id),
 CHECK(hour_start=date_trunc('hour',hour_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
 CHECK(paid_until=hour_start+INTERVAL '1 hour'),
 CHECK(charged_from>=hour_start AND charged_from<paid_until)
);
CREATE INDEX hourly_settlements_cost ON hourly_settlements(charged_from,enrollment_id);

CREATE TABLE hourly_account_preferences (
 owner_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 extra_enabled BIGINT NOT NULL DEFAULT 0 CHECK(extra_enabled IN(0,1)),
 accepted_extra_price_minor BIGINT CHECK(accepted_extra_price_minor>0),
 updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(owner_id,account_generation),
 CHECK(extra_enabled=0 OR accepted_extra_price_minor IS NOT NULL)
);

CREATE TABLE hourly_extra_purchases (
 id TEXT COLLATE "C" PRIMARY KEY,
 owner_id TEXT COLLATE "C" NOT NULL,
 account_generation BIGINT NOT NULL CHECK(account_generation>=0),
 day_utc DATE NOT NULL,
 block_index BIGINT NOT NULL CHECK(block_index>0),
 byte_count BIGINT NOT NULL CHECK(byte_count=1000000000),
 unit_price_minor BIGINT NOT NULL CHECK(unit_price_minor>0),
 policy_version TEXT COLLATE "C" NOT NULL CHECK(policy_version ~ '^[0-9a-f]{64}$'),
 ledger_id TEXT COLLATE "C" NOT NULL REFERENCES ledger_entries(id),
 created_at TIMESTAMPTZ NOT NULL,
 FOREIGN KEY(owner_id,account_generation,day_utc) REFERENCES hourly_credit_days(owner_id,account_generation,day_utc),
 CONSTRAINT hourly_extra_purchases_block UNIQUE(owner_id,account_generation,day_utc,block_index),
 CONSTRAINT hourly_extra_purchases_ledger UNIQUE(ledger_id)
);
CREATE INDEX hourly_extra_purchases_owner ON hourly_extra_purchases(owner_id,account_generation,day_utc,block_index);

-- +goose StatementBegin
CREATE FUNCTION purchase_term_snapshot_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.purchased_term_seconds IS DISTINCT FROM OLD.purchased_term_seconds THEN
  RAISE EXCEPTION 'original purchased validity is immutable';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER purchase_term_snapshot_guard BEFORE UPDATE OF purchased_term_seconds ON purchases FOR EACH ROW EXECUTE FUNCTION purchase_term_snapshot_guard_fn();

-- +goose StatementBegin
CREATE FUNCTION hourly_enrollment_scope_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.id,NEW.owner_id,NEW.account_generation,NEW.group_id,NEW.activation_key,NEW.request_fingerprint,NEW.created_at)
  IS DISTINCT FROM ROW(OLD.id,OLD.owner_id,OLD.account_generation,OLD.group_id,OLD.activation_key,OLD.request_fingerprint,OLD.created_at)
  OR (OLD.proxy_user_id IS NOT NULL AND ROW(NEW.proxy_user_id,NEW.period_id) IS DISTINCT FROM ROW(OLD.proxy_user_id,OLD.period_id))
  OR (OLD.activated_at IS NOT NULL AND NEW.activated_at IS DISTINCT FROM OLD.activated_at)
  OR (OLD.paid_through IS NOT NULL AND (NEW.paid_through IS NULL OR NEW.paid_through<OLD.paid_through))
  OR (OLD.status IN('funding_suspended','disabled','closed') AND NEW.status IN('preparing','active'))
 THEN RAISE EXCEPTION 'hourly activation scope and paid history are immutable'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER hourly_enrollment_scope_guard BEFORE UPDATE ON hourly_enrollments FOR EACH ROW EXECUTE FUNCTION hourly_enrollment_scope_guard_fn();

-- +goose StatementBegin
CREATE FUNCTION hourly_financial_immutable_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'hourly financial receipts are immutable';
END $$;
-- +goose StatementEnd
CREATE TRIGGER hourly_settlement_immutable BEFORE UPDATE OR DELETE ON hourly_settlements FOR EACH ROW EXECUTE FUNCTION hourly_financial_immutable_fn();
CREATE TRIGGER hourly_extra_purchase_immutable BEFORE UPDATE OR DELETE ON hourly_extra_purchases FOR EACH ROW EXECUTE FUNCTION hourly_financial_immutable_fn();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
