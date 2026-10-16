package database

import "time"

// AdminEntitlementEditInput is a validated full replacement of mutable fields.
type AdminEntitlementEditInput struct {
	ActorUserID, UserID, PurchaseID            string
	IdempotencyKey, RequestFingerprint, Reason string
	ExpectedUpdatedAt                          time.Time
	ComboID, Status, ResetStrategy             string
	ValidFrom, ValidUntil                      time.Time
	TrafficLimitBytes                          int64
	SquadUUIDs                                 []string
}

// AdminEntitlementRefundInput requests one exact local credit and remote sync.
type AdminEntitlementRefundInput struct {
	ActorUserID, UserID, PurchaseID            string
	IdempotencyKey, RequestFingerprint, Reason string
	AmountTXBMinor                             int64
}

// AdminCouponGrantInput adds one purchase-discount grant through an audited admin command.
type AdminCouponGrantInput struct {
	ActorUserID, UserID, CouponID, IdempotencyKey, Reason string
}

// AdminTemporaryBan is the current durable manual access restriction.
type AdminTemporaryBan struct {
	UserID           string     `json:"userId"`
	ActorUserID      string     `json:"actorUserId"`
	Reason           string     `json:"reason"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	UnbanOperationID string     `json:"unbanOperationId"`
	RestoredAt       *time.Time `json:"restoredAt"`
}

// AdminTemporaryBanInput starts one user-scoped restriction through the provider queue.
type AdminTemporaryBanInput struct {
	ActorUserID, UserID, IdempotencyKey, RequestFingerprint, Reason string
	DurationMinutes                                                 int
}

// AdminComboReplacementInput changes configuration without moving TXB.
type AdminComboReplacementInput struct {
	ActorUserID, UserID, ComboID               string
	IdempotencyKey, RequestFingerprint, Reason string
	AddonSquadUUIDs                            []string
}

// AdminBulkExtensionFilter uses inclusive OR matching across both dimensions.
type AdminBulkExtensionFilter struct {
	ComboIDs        []string
	AddonSquadUUIDs []string
}

// AdminBulkExtensionInput queues one deduplicated active-user extension.
type AdminBulkExtensionInput struct {
	ActorUserID, IdempotencyKey, RequestFingerprint, Reason string
	Filter                                                  AdminBulkExtensionFilter
	DurationMinutes                                         int
}

// AdminBulkExtensionPreview summarizes the current deduplicated target set.
type AdminBulkExtensionPreview struct {
	MatchedUsers     int `json:"matchedUsers"`
	ActivePurchases  int `json:"activePurchases"`
	QueuedSuccessors int `json:"queuedSuccessors"`
}

type adminBulkTarget struct {
	UserID, PurchaseID string
}
