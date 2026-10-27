-- +goose Up
CREATE INDEX pricing_offers_product_window ON pricing_offers(product_type,product_id,exposed_at DESC,id DESC);
CREATE INDEX pricing_features_entity_day ON pricing_features(entity_type,entity_id,day DESC);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
