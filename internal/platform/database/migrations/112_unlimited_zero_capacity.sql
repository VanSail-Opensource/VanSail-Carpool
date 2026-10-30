CREATE TABLE pricing_cost_metadata_replacement (
 node_id TEXT PRIMARY KEY REFERENCES xnode_nodes(id), cycle_cost_minor INTEGER NOT NULL CHECK(cycle_cost_minor>=0),
 cycle_days INTEGER NOT NULL CHECK(cycle_days BETWEEN 1 AND 366),
 capacity_bytes INTEGER NOT NULL CHECK(capacity_bytes>=0), effective_at TEXT NOT NULL,
 updated_by TEXT NOT NULL, updated_at TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
 charge_mode TEXT NOT NULL DEFAULT 'cycle' CHECK(charge_mode IN ('cycle','hourly')),
 hourly_rate_micros INTEGER NOT NULL DEFAULT 0 CHECK(hourly_rate_micros>=0),
 traffic_rate_micros INTEGER NOT NULL DEFAULT 0 CHECK(traffic_rate_micros>=0),
 capacity_unlimited INTEGER NOT NULL DEFAULT 0 CHECK(capacity_unlimited IN (0,1)),
 CHECK(capacity_bytes>0 OR capacity_unlimited=1)
);
INSERT INTO pricing_cost_metadata_replacement SELECT node_id,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,updated_by,updated_at,version,charge_mode,hourly_rate_micros,traffic_rate_micros,capacity_unlimited FROM pricing_cost_metadata;
DROP TABLE pricing_cost_metadata;
ALTER TABLE pricing_cost_metadata_replacement RENAME TO pricing_cost_metadata;
