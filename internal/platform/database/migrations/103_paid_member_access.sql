-- Keep paid PAYG eligibility when a combo changes. Around TX is gated on reads.
DROP TRIGGER IF EXISTS preference_visibility_before_purchase;
DROP TRIGGER IF EXISTS preference_visibility_before_transition;
DROP TRIGGER IF EXISTS preference_visibility_after_transition;

CREATE TRIGGER preference_visibility_before_purchase
BEFORE INSERT ON purchases
BEGIN
  UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.created_at)
      AND julianday(valid_until)>julianday(NEW.created_at)
  );
END;

CREATE TRIGGER preference_visibility_before_transition
BEFORE UPDATE OF status,valid_from,valid_until ON purchases
BEGIN
  UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
END;

CREATE TRIGGER preference_visibility_after_transition
AFTER UPDATE OF status,valid_from,valid_until ON purchases
BEGIN
  UPDATE user_preferences SET show_activity=0
  WHERE user_id=NEW.user_id
    AND NOT EXISTS (SELECT 1 FROM xnode_payg_accounts WHERE user_id=NEW.user_id AND purchased_bytes>0)
    AND NOT EXISTS (
    SELECT 1 FROM purchases WHERE user_id=NEW.user_id AND status='active'
      AND julianday(valid_from)<=julianday(NEW.updated_at)
      AND julianday(valid_until)>julianday(NEW.updated_at)
  );
END;

-- Enable the entrance for existing spenders once; future manual choices remain saved.
INSERT INTO user_preferences(user_id,show_around_tx)
SELECT DISTINCT user_id,1 FROM ledger_entries
WHERE delta_txb_minor<0 AND kind IN ('purchase_debit','purchase_addon_debit','automatic_renewal','payg_gb')
ON CONFLICT(user_id) DO UPDATE SET show_around_tx=1;
