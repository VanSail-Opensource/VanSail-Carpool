package purchaseops

import "strings"

// IsAutomaticTrafficResetKey identifies historical scanner-owned receipts only.
// No new reset command or automatic debit can be created.
func IsAutomaticTrafficResetKey(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "automatic-traffic-reset:")
}
