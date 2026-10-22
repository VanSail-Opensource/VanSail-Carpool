-- Identity migration temporarily rewrites owner references through the alias
-- table. Audit history remains immutable for every other update, including
-- changes made after the rekey has completed.
DROP TRIGGER IF EXISTS affiliate_config_versions_no_update;
CREATE TRIGGER affiliate_config_versions_no_update
BEFORE UPDATE ON affiliate_config_versions
WHEN NOT (
  OLD.id IS NEW.id
  AND OLD.version IS NEW.version
  AND OLD.created_at IS NEW.created_at
  AND EXISTS (
    SELECT 1 FROM member_identity_aliases a
    WHERE (OLD.created_by=a.old_user_id
           AND NEW.created_by IN (a.user_id,'~tg:'||a.user_id))
       OR (OLD.created_by='~tg:'||a.user_id AND NEW.created_by=a.user_id)
  )
)
BEGIN
  SELECT RAISE(ABORT,'affiliate config history is immutable');
END;

DROP TRIGGER IF EXISTS user_notification_events_immutable;
CREATE TRIGGER user_notification_events_immutable
BEFORE UPDATE OF user_id,source_kind,source_id,kind,payload_json,gate_key,created_at
ON user_notification_events
WHEN NOT (
  OLD.source_kind IS NEW.source_kind
  AND OLD.source_id IS NEW.source_id
  AND OLD.kind IS NEW.kind
  AND OLD.payload_json IS NEW.payload_json
  AND OLD.gate_key IS NEW.gate_key
  AND OLD.created_at IS NEW.created_at
  AND EXISTS (
    SELECT 1 FROM member_identity_aliases a
    WHERE (OLD.user_id=a.old_user_id
           AND NEW.user_id IN (a.user_id,'~tg:'||a.user_id))
       OR (OLD.user_id='~tg:'||a.user_id AND NEW.user_id=a.user_id)
  )
)
BEGIN
  SELECT RAISE(ABORT,'user notification payloads are immutable');
END;
