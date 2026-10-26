package database

import (
	"crypto/sha256"
	"encoding/binary"
	"strconv"
)

// Forecasts use compact positive bigint identifiers on disk. They are decimal
// strings in browser/export transport. Identity conflicts fail closed rather
// than overwriting a vintage; these identifiers never authorize financial work.
func pricingForecastID(id string) string {
	if n, err := strconv.ParseInt(id, 10, 64); err == nil && n > 0 {
		return strconv.FormatInt(n, 10)
	}
	h := sha256.Sum256([]byte(id))
	n := binary.BigEndian.Uint64(h[:8]) & uint64(1<<63-1)
	if n == 0 {
		n = 1
	}
	return strconv.FormatUint(n, 10)
}
