-- Explicitly reconciled historical report references, scoped to their source node.
CREATE TABLE xnode_period_aliases (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id),
 historical_period_id TEXT NOT NULL,
 canonical_period_id TEXT NOT NULL REFERENCES xnode_periods(id),
 PRIMARY KEY(node_id,historical_period_id),
 CHECK(historical_period_id <> canonical_period_id)
);
