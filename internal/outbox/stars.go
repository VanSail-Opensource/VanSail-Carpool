package outbox

// StarsReconciliationKind advances one bounded page of authoritative Stars history.
const StarsReconciliationKind = "telegram_star_reconciliation"

// StarsReconciliationPage binds a job to one bot and durable cursor generation.
type StarsReconciliationPage struct {
	BotID      int64 `json:"botId"`
	Generation int64 `json:"generation"`
}
