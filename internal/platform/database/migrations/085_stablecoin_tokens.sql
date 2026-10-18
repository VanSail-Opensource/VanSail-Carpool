-- Preserve adapter progress as USDT progress; do not rewrite evidence IDs or quotes.
CREATE TABLE stablecoin_cursors (
 network TEXT NOT NULL, token TEXT NOT NULL, value TEXT NOT NULL,
 hash TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(network,token)
);
INSERT INTO stablecoin_cursors(network,token,value,hash,updated_at)
SELECT network, CASE network
 WHEN 'tron' THEN 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t'
 WHEN 'ethereum' THEN '0xdac17f958d2ee523a2206206994597c13d831ec7'
 WHEN 'bsc' THEN '0x55d398326f99059ff775485246999027b3197955'
 WHEN 'polygon' THEN '0xc2132d05d31c914a87c6611c10748aeb04b58e8f'
 WHEN 'arbitrum' THEN '0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9'
 WHEN 'xlayer' THEN '0x779ded0c9e1022225f8e0630b35a9b54be713736'
 WHEN 'plasma' THEN '0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb'
 WHEN 'solana' THEN 'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB'
 WHEN 'aptos' THEN '0x357b0b74bc833e95a115ad22604854d6b0fca151cecd94111770e5d6ffc9dc2b'
 WHEN 'ton' THEN '0:b113a994b5024a16719f69139328eb759596c38a25f59028b146fecdc3621dfe'
 END, value,hash,updated_at FROM usdt_scan_cursors;
DROP TABLE usdt_scan_cursors;
ALTER TABLE stablecoin_cursors RENAME TO usdt_scan_cursors;

CREATE TABLE stablecoin_transfers (
 id TEXT PRIMARY KEY, network TEXT NOT NULL, token TEXT NOT NULL, recipient TEXT NOT NULL,
 atomic_amount TEXT NOT NULL, amount TEXT NOT NULL, tx_hash TEXT NOT NULL, event_id TEXT NOT NULL,
 occurred_at TEXT NOT NULL, order_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('ready','review','credited','dismissed')),
 reason TEXT NOT NULL DEFAULT '', received_at TEXT NOT NULL,
 dismissed_by TEXT, dismissed_reason TEXT, dismissed_at TEXT,
 UNIQUE(network,tx_hash,event_id),
 CHECK((status='dismissed')=(dismissed_by IS NOT NULL AND dismissed_reason IS NOT NULL AND dismissed_at IS NOT NULL))
);
INSERT INTO stablecoin_transfers SELECT *,NULL,NULL,NULL FROM usdt_transfers;
DROP TABLE usdt_transfers;
ALTER TABLE stablecoin_transfers RENAME TO usdt_transfers;
CREATE INDEX usdt_transfers_review_idx ON usdt_transfers(status,received_at,id);
CREATE INDEX usdt_transfers_match_idx ON usdt_transfers(network,token,recipient,status);
CREATE TRIGGER usdt_terminal_evidence_immutable BEFORE UPDATE ON usdt_transfers
WHEN OLD.status IN ('credited','dismissed') BEGIN
 SELECT RAISE(ABORT,'terminal transfer evidence is immutable');
END;
