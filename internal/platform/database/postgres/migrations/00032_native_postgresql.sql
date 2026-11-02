-- +goose Up
-- All existing migrations and text evidence remain immutable. Replace live
-- compatibility dependencies before removing functions, without CASCADE.
-- Only absolute ISO dates are accepted. Fixed parser settings make this safe in
-- expression indexes; relative inputs such as 'now' must never be indexed.
-- +goose StatementBegin
CREATE FUNCTION vansail_timestamp(value text) RETURNS timestamptz LANGUAGE plpgsql IMMUTABLE STRICT
SET TimeZone='UTC' SET DateStyle='ISO,YMD' AS $$
BEGIN
 IF value !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}([T ][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})?)?$' THEN RETURN NULL; END IF;
 RETURN value::timestamptz;
 EXCEPTION WHEN invalid_datetime_format OR datetime_field_overflow THEN RETURN NULL;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $native$
DECLARE item record; expression text;
BEGIN
 FOR item IN SELECT c.oid,c.conname,c.conrelid::regclass AS relation,
   pg_get_constraintdef(c.oid) AS definition FROM pg_constraint c
   JOIN pg_namespace n ON n.oid=c.connamespace
   WHERE n.nspname='public' AND c.contype='c'
     AND pg_get_constraintdef(c.oid) ~ 'json_(valid|type)\('
 LOOP
  expression := regexp_replace(item.definition,
    'json_valid\(([a-z_][a-z_0-9]*)\)', 'COALESCE((\1 IS JSON),false)', 'g');
  expression := regexp_replace(expression,
    'json_type\(([a-z_][a-z_0-9]*)\)', 'jsonb_typeof(\1::jsonb)', 'g');
  EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I',item.relation,item.conname);
  EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s',item.relation,item.conname,expression);
 END LOOP;
END $native$;
-- +goose StatementEnd

DROP INDEX ip_lookup_cache;
CREATE INDEX ip_lookup_cache ON ip_lookup_reports(ip,vansail_timestamp(completed_at) DESC);
DROP INDEX telegram_pm_profile_attempt_idx;
CREATE UNIQUE INDEX telegram_pm_profile_attempt_idx ON provider_operation_items(
 (CASE WHEN target_type='pm_topic_recovery' THEN substr(target_id,1,strpos(target_id,':')-1) ELSE target_id END))
 WHERE status='processing' AND ((target_type='pm_conversation' AND item_key='profile') OR target_type='pm_topic_recovery');

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION "preference_visibility_after_transition_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND vansail_timestamp(valid_from)<=vansail_timestamp(NEW.updated_at)
      AND vansail_timestamp(valid_until)>vansail_timestamp(NEW.updated_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION "preference_visibility_before_purchase_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND vansail_timestamp(valid_from)<=vansail_timestamp(NEW.created_at)
      AND vansail_timestamp(valid_until)>vansail_timestamp(NEW.created_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION "preference_visibility_before_transition_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND vansail_timestamp(valid_from)<=vansail_timestamp(NEW.updated_at)
      AND vansail_timestamp(valid_until)>vansail_timestamp(NEW.updated_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd


DROP FUNCTION json_valid(text),json_type(text),instr(text,text),json_extract(text,text),json_each(text),
 vansail_json(text),json_set(text,text,jsonb),json_patch(text,text),julianday(text);
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'VanSail migrations are forward-only'; END $$;
-- +goose StatementEnd
