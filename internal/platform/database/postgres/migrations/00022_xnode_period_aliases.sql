-- +goose Up
CREATE TABLE xnode_period_aliases (
 node_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_nodes(id),
 historical_period_id TEXT COLLATE "C" NOT NULL,
 canonical_period_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_periods(id),
 PRIMARY KEY(node_id,historical_period_id),
 CHECK(historical_period_id <> canonical_period_id)
);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
