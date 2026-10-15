package usdt

// pruneSolanaOwners preserves every discovered (including closed) account for
// watched owners. The watch set includes unpaid late-payment recipients.
func pruneSolanaOwners(cursor *solCursor, owners []string) {
	watched := make(map[string]bool, len(owners))
	for _, owner := range owners {
		watched[owner] = true
	}
	stale := false
	for _, history := range cursor.Accounts {
		if !watched[history.Owner] {
			stale = true
			break
		}
	}
	if !stale {
		return
	}
	compact := make(map[string]solHistory)
	for account, history := range cursor.Accounts {
		if watched[history.Owner] {
			compact[account] = history
		}
	}
	cursor.Accounts = compact
	cursor.Position = 0
}
