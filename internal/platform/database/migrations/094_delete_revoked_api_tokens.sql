-- Revoked credentials have no runtime purpose; immutable audit events remain.
DELETE FROM admin_api_tokens WHERE revoked_at IS NOT NULL;
