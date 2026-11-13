package database

// statisticTimeRangeSQL selects [from,to) from canonical stamp-owned TEXT.
// The coarse raw bounds retain indexed candidates; the normalized comparison
// resolves variable fractional widths without rounding nanoseconds in SQL.
// All arguments are fixed SQL expressions, never request-provided strings.
func statisticTimeRangeSQL(column, from, to string) string {
	return column + ">=left(" + from + ",19) AND " + column + "<=(left(" + to + ",19)||'Z') AND " +
		purchaseTimestampOrderSQL(column) + ">=" + from + " AND " + purchaseTimestampOrderSQL(column) + "<" + to
}
