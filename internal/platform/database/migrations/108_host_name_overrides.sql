-- Owner-authored display choices cannot be derived from published inventory.
-- Virtual PAYG/split IDs deliberately do not reference physical endpoint rows.
CREATE TABLE user_host_names (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 host_id TEXT NOT NULL CHECK(length(host_id) BETWEEN 1 AND 256),
 name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 PRIMARY KEY(user_id,host_id)
);
