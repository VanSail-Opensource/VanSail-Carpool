package outbox

// ContinuityKind extends the current upstream entitlement before a queued term
// reaches its local activation boundary.
const ContinuityKind = "remna_prepare_continuity"

// PaymentSuccessAnnouncementKind sends one durable channel announcement for
// the authoritative first transition of a provider payment to paid.
const PaymentSuccessAnnouncementKind = "telegram_payment_success_announcement"

const AffiliateSuccessKind = "telegram_affiliate_success"
const AffiliateTierUpgradeKind = "telegram_affiliate_tier_upgrade"

// UserNotificationKind delivers one immutable private-chat user event.
const UserNotificationKind = "telegram_user_notification"

const (
	UserEventExpiration                 = "expiration"
	UserEventExpiryReminder             = "expiry_reminder"
	UserEventQueuedActivation           = "queued_activation"
	UserEventPurchaseActivation         = "purchase_activation"
	UserEventAutoRenewal                = "auto_renewal"
	UserEventPaymentCredited            = "payment_credited"
	UserEventPaymentRefunded            = "payment_refunded"
	UserEventTrafficThreshold           = "traffic_threshold"
	UserEventAutomaticReset             = "automatic_traffic_reset"
	UserEventAutomaticResetInsufficient = "automatic_traffic_reset_insufficient"
	UserEventAutomaticResetFailed       = "automatic_traffic_reset_failed"
	UserEventGroupReward                = "group_reward"
	UserEventAdminExtension             = "admin_extension"
	UserEventAdminUpdate                = "admin_update"
	UserEventNodeCompensation           = "node_compensation"
	UserEventPurchaseQueued             = "purchase_queued"
	UserEventRenewalScheduled           = "renewal_scheduled"
	UserEventAddonActivated             = "addon_activated"
	UserEventQueuedCancellation         = "queued_cancellation"
	UserEventAutoRenewalFailed          = "auto_renewal_failed"
	UserEventManualResetCompleted       = "manual_reset_completed"
	UserEventManualResetRefunded        = "manual_reset_refunded"
	UserEventMemberRefundCompleted      = "member_refund_completed"
	UserEventMemberRefundFailed         = "member_refund_failed"
	UserEventProvisionConflict          = "remnawave_username_conflict_refund"
)
