-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION "preference_visibility_after_transition_fn"() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
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
      AND julianday(valid_from)<=julianday(NEW.created_at)
      AND julianday(valid_until)>julianday(NEW.created_at)
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
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
IF TG_OP='DELETE' THEN RETURN OLD; END IF;
RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd

-- Enable the entrance for existing spenders once; future manual choices remain saved.
INSERT INTO user_preferences(user_id,show_around_tx)
SELECT DISTINCT user_id,1 FROM ledger_entries
WHERE delta_txb_minor<0 AND kind IN ('purchase_debit','purchase_addon_debit','automatic_renewal','payg_gb')
ON CONFLICT(user_id) DO UPDATE SET show_around_tx=1;

INSERT INTO schema_migrations(version,applied_at) VALUES('103_paid_member_access.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'VanSail migrations are forward-only'; END $$;
-- +goose StatementEnd
