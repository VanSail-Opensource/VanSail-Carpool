package database

import "time"

// Purchase timestamps are written by stamp in canonical UTC RFC3339 format.
// Normalize fractional precision without rounding to PostgreSQL microseconds;
// retained stamps may contain equivalent trailing zeroes. Only fixed internal
// column names and placeholder expressions may enter this SQL fragment.
func purchaseTimestampOrderSQL(column string) string {
	return `rpad(rtrim(` + column + `,'Z') || CASE WHEN strpos(` + column + `,'.')=0 THEN '.' ELSE '' END,29,'0') COLLATE "C"`
}

func purchaseTimestampOrder(at time.Time) string {
	return at.UTC().Format("2006-01-02T15:04:05.000000000")
}

// PostgreSQL clocks have microsecond precision. Format that exact UTC instant
// on the same scale as stamp values instead of rounding the retained deadline.
const databaseClockTimestampOrderSQL = `(to_char(clock_timestamp() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US') || '000') COLLATE "C"`
