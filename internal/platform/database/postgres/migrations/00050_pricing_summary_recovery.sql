-- +goose Up
-- Compacted summaries retain their original metrics. Late facts mark those
-- metrics stale; partial raw history can never replace a complete aggregate.
ALTER TABLE pricing_calibration_daily
 ADD COLUMN sealed BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN stale BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN dirty BOOLEAN NOT NULL DEFAULT false,
 ADD CONSTRAINT pricing_calibration_stale_requires_sealed CHECK(NOT stale OR sealed),
 ADD CONSTRAINT pricing_calibration_dirty_requires_unsealed CHECK(NOT dirty OR NOT sealed);
ALTER TABLE pricing_model_validation_daily
 ADD COLUMN sealed BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN stale BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN dirty BOOLEAN NOT NULL DEFAULT false,
 ADD CONSTRAINT pricing_validation_stale_requires_sealed CHECK(NOT stale OR sealed),
 ADD CONSTRAINT pricing_validation_dirty_requires_unsealed CHECK(NOT dirty OR NOT sealed);

-- Old raw retention could already have removed any subset of these cohorts.
-- Keep the evidence, make its uncertainty explicit, and rebuild only cohorts
-- whose complete source is still guaranteed by the former retention windows.
UPDATE pricing_calibration_daily SET sealed=true,stale=true
 WHERE day<=to_char((clock_timestamp() AT TIME ZONE 'UTC')::date-60,'YYYY-MM-DD');
UPDATE pricing_model_validation_daily SET sealed=true,stale=true
 WHERE day<=to_char((clock_timestamp() AT TIME ZONE 'UTC')::date-30,'YYYY-MM-DD');
UPDATE pricing_calibration_daily SET dirty=true WHERE NOT sealed;
UPDATE pricing_model_validation_daily SET dirty=true WHERE NOT sealed;

-- +goose StatementBegin
CREATE FUNCTION pricing_summary_sealed_guard_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF OLD.sealed AND ((to_jsonb(NEW)-'stale') IS DISTINCT FROM (to_jsonb(OLD)-'stale')
                   OR (OLD.stale AND NOT NEW.stale)) THEN
  RAISE EXCEPTION USING MESSAGE='sealed pricing summary cannot be rewritten or reopened', ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER pricing_calibration_sealed_guard BEFORE UPDATE ON pricing_calibration_daily
 FOR EACH ROW EXECUTE FUNCTION pricing_summary_sealed_guard_fn();
CREATE TRIGGER pricing_validation_sealed_guard BEFORE UPDATE ON pricing_model_validation_daily
 FOR EACH ROW EXECUTE FUNCTION pricing_summary_sealed_guard_fn();

-- Writers take a shared gate before locking any source tuple. Summarizers and
-- source-only compaction use a nonblocking exclusive gate, so they never queue
-- ahead of another billing writer that already holds financial row locks.
-- +goose StatementBegin
CREATE FUNCTION pricing_calibration_write_lock_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 PERFORM pg_advisory_xact_lock_shared(hashtextextended('pricing.calibration',0));
 RETURN NULL;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER pricing_offer_calibration_lock
 BEFORE INSERT OR UPDATE OF exposed_at,expires_at,model_version,predicted_probability,outcome,outcome_reason,observed_at
 ON pricing_offers FOR EACH STATEMENT EXECUTE FUNCTION pricing_calibration_write_lock_fn();
CREATE TRIGGER pricing_quote_calibration_lock
 BEFORE INSERT OR UPDATE OF generated_at,valid_until,model_version,probability,state,outcome_reason,authorized_at
 ON pricing_renewal_quotes FOR EACH STATEMENT EXECUTE FUNCTION pricing_calibration_write_lock_fn();

-- +goose StatementBegin
CREATE FUNCTION pricing_calibration_invalidate(cohort_day TEXT,cohort_model TEXT,cohort_kind TEXT)
 RETURNS void LANGUAGE plpgsql AS $vansail$
BEGIN
 UPDATE pricing_calibration_daily SET stale=true
  WHERE day=cohort_day AND model_version=cohort_model AND kind=cohort_kind AND sealed AND NOT stale;
 UPDATE pricing_calibration_daily SET dirty=true
  WHERE day=cohort_day AND model_version=cohort_model AND kind=cohort_kind AND NOT sealed AND NOT dirty;
END;
$vansail$;
-- +goose StatementEnd

-- AFTER ROW observes actual mutations, not a conflict-ignored INSERT or a
-- no-op assignment. Retention DELETE deliberately does not invalidate totals.
-- +goose StatementBegin
CREATE FUNCTION pricing_offer_calibration_changed_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF ROW(OLD.exposed_at,OLD.expires_at,OLD.model_version,OLD.predicted_probability,OLD.outcome,OLD.outcome_reason,OLD.observed_at)
   IS NOT DISTINCT FROM
     ROW(NEW.exposed_at,NEW.expires_at,NEW.model_version,NEW.predicted_probability,NEW.outcome,NEW.outcome_reason,NEW.observed_at) THEN
   RETURN NEW;
  END IF;
  IF OLD.outcome IS NOT NULL AND OLD.outcome_reason<>'INSUFFICIENT_FUNDS' THEN
   PERFORM pricing_calibration_invalidate(substr(OLD.exposed_at,1,10),OLD.model_version,'purchase');
  END IF;
 END IF;
 IF NEW.outcome IS NOT NULL AND NEW.outcome_reason<>'INSUFFICIENT_FUNDS' THEN
  PERFORM pricing_calibration_invalidate(substr(NEW.exposed_at,1,10),NEW.model_version,'purchase');
 END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER pricing_offer_calibration_changed
 AFTER INSERT OR UPDATE OF exposed_at,expires_at,model_version,predicted_probability,outcome,outcome_reason,observed_at
 ON pricing_offers FOR EACH ROW EXECUTE FUNCTION pricing_offer_calibration_changed_fn();

-- +goose StatementBegin
CREATE FUNCTION pricing_quote_calibration_changed_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 IF TG_OP='UPDATE' THEN
  -- Operational state/reason spelling and a non-null authorization refresh
  -- do not change the retained zero/one calibration contribution.
  IF ROW(OLD.generated_at,OLD.valid_until,OLD.model_version,OLD.probability,
         OLD.state<>'CANCELLED' AND OLD.authorized_at IS NOT NULL AND OLD.outcome_reason<>'INSUFFICIENT_FUNDS',OLD.state='SETTLED')
   IS NOT DISTINCT FROM
     ROW(NEW.generated_at,NEW.valid_until,NEW.model_version,NEW.probability,
         NEW.state<>'CANCELLED' AND NEW.authorized_at IS NOT NULL AND NEW.outcome_reason<>'INSUFFICIENT_FUNDS',NEW.state='SETTLED') THEN
   RETURN NEW;
  END IF;
  IF OLD.state<>'CANCELLED' AND OLD.authorized_at IS NOT NULL AND OLD.outcome_reason<>'INSUFFICIENT_FUNDS' THEN
   PERFORM pricing_calibration_invalidate(substr(OLD.generated_at,1,10),OLD.model_version,'renewal');
  END IF;
 END IF;
 IF NEW.state<>'CANCELLED' AND NEW.authorized_at IS NOT NULL AND NEW.outcome_reason<>'INSUFFICIENT_FUNDS' THEN
  PERFORM pricing_calibration_invalidate(substr(NEW.generated_at,1,10),NEW.model_version,'renewal');
 END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER pricing_quote_calibration_changed
 AFTER INSERT OR UPDATE OF generated_at,valid_until,model_version,probability,state,outcome_reason,authorized_at
 ON pricing_renewal_quotes FOR EACH ROW EXECUTE FUNCTION pricing_quote_calibration_changed_fn();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
