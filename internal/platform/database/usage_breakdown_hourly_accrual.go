package database

// Legacy receipts retain their inclusive ledger-time attribution. New receipts
// recognize only elapsed enabled time, including purchases made before the
// rolling window. Cumulative integer allocation makes adjacent windows additive
// and bounds the lifetime total by the immutable debit. PostgreSQL numeric div
// keeps that allocation exact, including nanosecond request bounds and totals
// above int64. The typed receipt timestamps themselves retain microseconds.
var usageBreakdownHourlyAccrualSQL = `
 SELECT e.group_id,CASE WHEN s.minimum_hours IS NULL THEN s.charged_minor::numeric
  ELSE div(s.charged_minor::numeric*bounds.end_ns,span.total_ns)
      -div(s.charged_minor::numeric*bounds.start_ns,span.total_ns) END actual
 FROM hourly_enrollments e JOIN hourly_settlements s ON s.enrollment_id=e.id
 JOIN ledger_entries l ON l.id=s.ledger_id AND l.user_id=e.owner_id AND l.kind='hourly_charge'
 CROSS JOIN LATERAL (
  SELECT EXTRACT(EPOCH FROM (s.paid_until-s.charged_from))*1000000000 total_ns
 ) span
 CROSS JOIN LATERAL (
  SELECT GREATEST(0,EXTRACT(EPOCH FROM ($8::timestamptz-s.charged_from))*1000000000+$9::integer) start_ns,
   LEAST(span.total_ns,
    EXTRACT(EPOCH FROM ($7::timestamptz-s.charged_from))*1000000000+$10::integer,
    EXTRACT(EPOCH FROM (COALESCE(e.disabled_at,s.paid_until)-s.charged_from))*1000000000) end_ns
 ) bounds
 WHERE e.owner_id=$1 AND e.account_generation=$2 AND (
  (s.minimum_hours IS NULL AND ` + purchaseTimestampOrderSQL("l.created_at") + ` BETWEEN $3 AND $4)
  OR (s.minimum_hours IS NOT NULL AND s.charged_from<=$7 AND s.paid_until>$8 AND bounds.end_ns>bounds.start_ns))`
