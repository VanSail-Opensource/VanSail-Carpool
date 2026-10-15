CREATE TABLE admin_api_tokens (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 80),
 token_prefix TEXT NOT NULL,
 token_hash BLOB NOT NULL UNIQUE CHECK(length(token_hash)=32),
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 created_at TEXT NOT NULL,
 expires_at TEXT,
 last_used_at TEXT,
 revoked_at TEXT
);
CREATE INDEX admin_api_tokens_owner ON admin_api_tokens(user_id,created_at DESC);
