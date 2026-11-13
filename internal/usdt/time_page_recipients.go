package usdt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// bindTimePage restarts the same window when its ordered query recipients change.
// A provider fingerprint or numeric offset is valid only for the original query.
// Re-reading the window is safe because receiving-event identity is durable.
func bindTimePage(page timePage, recipients []string) timePage {
	digest := sha256.Sum256([]byte(strings.Join(recipients, "\x00")))
	scope := hex.EncodeToString(digest[:])
	if page.Recipients != scope {
		page.Address, page.Offset, page.Fingerprint = 0, 0, ""
		page.Recipients = scope
	}
	return page
}
