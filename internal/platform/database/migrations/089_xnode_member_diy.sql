-- One encrypted client-only override document per member. No generated client
-- configurations, credentials or duplicate routing desired state are stored.
CREATE TABLE xnode_member_diy (
 user_id TEXT PRIMARY KEY REFERENCES users(id),
 revision INTEGER NOT NULL CHECK(revision>=1 AND revision<=9007199254740991),
 cipher TEXT NOT NULL
);
