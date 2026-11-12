-- +goose Up
-- Recover chronological Stars history in bounded pages. The prior transaction
-- hash detects shifted numeric offsets; no wallet or payment evidence changes.
CREATE TABLE telegram_star_reconciliation (
  bot_id BIGINT PRIMARY KEY CHECK (bot_id>0),
  generation BIGINT NOT NULL CHECK (generation>0),
  next_offset BIGINT NOT NULL CHECK (next_offset>=0 AND next_offset<=2147483647),
  anchor TEXT COLLATE "C" NOT NULL,
  available_at TEXT COLLATE "C" NOT NULL,
  updated_at TEXT COLLATE "C" NOT NULL,
  CHECK ((next_offset=0 AND anchor='') OR (next_offset>0 AND anchor ~ '^[0-9a-f]{64}$'))
);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
