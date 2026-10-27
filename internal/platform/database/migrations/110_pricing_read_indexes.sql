-- Scope closed response observations before ranking distinct member/day outcomes.
CREATE INDEX pricing_offers_product_window ON pricing_offers(product_type,product_id,exposed_at DESC,id DESC);
-- Daily feature and owner projections must not scan retained compute snapshots.
CREATE INDEX pricing_features_entity_day ON pricing_features(entity_type,entity_id,day DESC);
