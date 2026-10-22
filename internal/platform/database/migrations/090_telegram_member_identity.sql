-- Upgrade aliases are required for queued payloads and late period-scoped traffic.
-- Member keys and encrypted documents are converted together after the vault is
-- available, before the application starts workers or accepts requests.
CREATE TABLE member_identity_aliases (
 old_user_id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 old_proxy_id INTEGER UNIQUE,
 proxy_id INTEGER UNIQUE,
 CHECK ((old_proxy_id IS NULL)=(proxy_id IS NULL))
);
