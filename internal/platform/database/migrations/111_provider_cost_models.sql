ALTER TABLE pricing_cost_metadata ADD COLUMN charge_mode TEXT NOT NULL DEFAULT 'cycle' CHECK(charge_mode IN ('cycle','hourly'));
ALTER TABLE pricing_cost_metadata ADD COLUMN hourly_rate_micros INTEGER NOT NULL DEFAULT 0 CHECK(hourly_rate_micros>=0);
ALTER TABLE pricing_cost_metadata ADD COLUMN traffic_rate_micros INTEGER NOT NULL DEFAULT 0 CHECK(traffic_rate_micros>=0);
ALTER TABLE pricing_cost_metadata ADD COLUMN capacity_unlimited INTEGER NOT NULL DEFAULT 0 CHECK(capacity_unlimited IN (0,1));
ALTER TABLE pricing_cost_versions ADD COLUMN charge_mode TEXT NOT NULL DEFAULT 'cycle' CHECK(charge_mode IN ('cycle','hourly'));
ALTER TABLE pricing_cost_versions ADD COLUMN hourly_rate_micros INTEGER NOT NULL DEFAULT 0 CHECK(hourly_rate_micros>=0);
ALTER TABLE pricing_cost_versions ADD COLUMN traffic_rate_micros INTEGER NOT NULL DEFAULT 0 CHECK(traffic_rate_micros>=0);
ALTER TABLE pricing_cost_versions ADD COLUMN capacity_unlimited INTEGER NOT NULL DEFAULT 0 CHECK(capacity_unlimited IN (0,1));
