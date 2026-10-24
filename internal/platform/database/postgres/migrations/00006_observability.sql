-- +goose Up
CREATE VIEW vansail_outbox_observability AS
 SELECT status,COUNT(*) AS jobs,COALESCE(SUM(attempts),0) AS attempts,
 COALESCE(MAX(extract(epoch FROM clock_timestamp()-created_at::timestamptz)),0) AS oldest_seconds
 FROM outbox_jobs WHERE status IN('pending','processing','failed') GROUP BY status;
CREATE VIEW vansail_financial_observability AS
 SELECT COUNT(*) AS discrepancies FROM balances b FULL JOIN
 (SELECT user_id,SUM(delta_txb_minor) AS total FROM ledger_entries GROUP BY user_id) l ON l.user_id=b.user_id
 WHERE b.user_id IS NULL OR b.txb_minor<>COALESCE(l.total,0);
CREATE VIEW vansail_accounting_observability AS
 SELECT COUNT(*) AS pending_period_nodes,
 COALESCE(SUM(GREATEST(COALESCE(p.flushed_sequence,0)-COALESCE(c.last_sequence,0),0)),0) AS known_sequence_backlog
 FROM xnode_period_nodes p JOIN xnode_periods period ON period.id=p.period_id
 JOIN xnode_nodes n ON n.id=p.node_id LEFT JOIN xnode_report_cursors c ON c.agent_id=n.agent_id
 WHERE period.status IN('closing','review') AND (p.flushed_sequence IS NULL OR p.flushed_sequence>COALESCE(c.last_sequence,0));
CREATE VIEW vansail_backup_observability AS
 SELECT COALESCE(MAX(completed_at::timestamptz) FILTER(WHERE status='complete'),'epoch'::timestamptz) AS last_complete,
 COUNT(*) FILTER(WHERE status='failed') AS failed_exports FROM backup_runs;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
