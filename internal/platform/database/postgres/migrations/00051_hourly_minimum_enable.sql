-- +goose Up
-- The existing hourly_offers.required_hours value now means minimum enable
-- hours. Keep its value and zero's encrypted-policy inheritance unchanged.
-- No old activation is retroactively charged and no financial fact is rewritten.

-- Replace only the old single-hour amount/interval checks, whose generated
-- names may differ across installations. Identity, uniqueness and immutability
-- constraints remain intact.
-- +goose StatementBegin
DO $$
DECLARE constraint_row RECORD;
BEGIN
 FOR constraint_row IN
  SELECT DISTINCT c.conname FROM pg_constraint c
  JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
  WHERE c.conrelid='hourly_settlements'::regclass AND c.contype='c'
   AND a.attname IN ('charged_minor','paid_until')
 LOOP
  EXECUTE format('ALTER TABLE hourly_settlements DROP CONSTRAINT %I',constraint_row.conname);
 END LOOP;
END $$;
-- +goose StatementEnd

-- NULL identifies pre-upgrade receipts, whose usage history remains on the
-- original ledger-time basis. New receipts always explicitly write 0 (renewal)
-- or a positive minimum duration (activation), for interval-based accrual.
ALTER TABLE hourly_settlements ADD COLUMN minimum_hours BIGINT
 CHECK(minimum_hours>=0 AND minimum_hours<=2562047);
ALTER TABLE hourly_settlements
 ADD CONSTRAINT hourly_settlements_positive_charge CHECK(charged_minor>0),
 ADD CONSTRAINT hourly_settlements_interval_start CHECK(
  charged_from>=hour_start AND charged_from<hour_start+INTERVAL '1 hour'),
 ADD CONSTRAINT hourly_settlements_purchased_interval CHECK(
  (COALESCE(minimum_hours,0)=0 AND charged_minor<=hourly_price_minor AND paid_until=hour_start+INTERVAL '1 hour')
  OR (minimum_hours>0 AND charged_minor::numeric=hourly_price_minor::numeric*minimum_hours
   AND paid_until=charged_from+minimum_hours*INTERVAL '1 hour'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
