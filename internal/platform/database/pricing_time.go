package database

import "time"

// Pricing timestamps preserve nanoseconds at fixed precision so TEXT indexes
// compare exact instants even when one value has trailing fractional zeroes.
// Existing immutable source timestamps are never rewritten.
func pricingStamp(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000000Z") }
