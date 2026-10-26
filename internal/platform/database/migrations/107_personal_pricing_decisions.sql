CREATE TABLE pricing_personal_decisions (
 version TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id),
 product_type TEXT NOT NULL CHECK (product_type IN ('combo','addon','payg')),
 product_id TEXT NOT NULL,
 context_key TEXT NOT NULL,
 policy_version INTEGER NOT NULL,
 public_minor INTEGER NOT NULL CHECK(public_minor>=0),
 personal_minor INTEGER NOT NULL CHECK(personal_minor>=0),
 net_minor INTEGER NOT NULL CHECK(net_minor>=0),
 status TEXT NOT NULL CHECK(status IN ('ADJUSTED','UNCHANGED')),
 reason TEXT NOT NULL,
 model_version TEXT NOT NULL,
 probability REAL,
 calibration TEXT NOT NULL,
 generated_at TEXT NOT NULL,
 valid_until TEXT NOT NULL,
 coupon_json TEXT NOT NULL CHECK(json_valid(coupon_json))
);
CREATE INDEX pricing_personal_owner_idx ON pricing_personal_decisions(user_id,product_type,product_id,generated_at);
ALTER TABLE pricing_offers ADD COLUMN outcome_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE pricing_renewal_quotes ADD COLUMN outcome_reason TEXT NOT NULL DEFAULT '';
