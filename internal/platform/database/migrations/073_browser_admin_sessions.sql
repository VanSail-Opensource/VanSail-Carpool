ALTER TABLE sessions ADD COLUMN scope TEXT NOT NULL DEFAULT 'miniapp' CHECK (scope IN ('miniapp','browser'));
ALTER TABLE sessions ADD COLUMN renewed_at TEXT NOT NULL DEFAULT '';
UPDATE sessions SET renewed_at=created_at;
DROP INDEX IF EXISTS sessions_user_id_unique_idx;
CREATE UNIQUE INDEX sessions_miniapp_user_unique_idx ON sessions(user_id) WHERE scope='miniapp';
CREATE INDEX sessions_scope_expiry_idx ON sessions(scope,expires_at);

CREATE TABLE browser_access_links (
  token_hash BLOB PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  return_to TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE INDEX browser_access_links_expiry_idx ON browser_access_links(expires_at);
