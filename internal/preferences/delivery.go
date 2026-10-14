package preferences

import (
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// Allows maps every supported event to exactly one notification category.
func (p Preferences) Allows(kind string) (bool, error) {
	switch kind {
	case outbox.UserEventExpiration, outbox.UserEventExpiryReminder, outbox.UserEventQueuedActivation,
		outbox.UserEventPurchaseActivation, outbox.UserEventAutoRenewal, outbox.UserEventPurchaseQueued,
		outbox.UserEventRenewalScheduled, outbox.UserEventAddonActivated, outbox.UserEventQueuedCancellation,
		outbox.UserEventAutoRenewalFailed:
		return p.Notifications.Combos, nil
	case outbox.UserEventTrafficThreshold, outbox.UserEventAutomaticReset, outbox.UserEventAutomaticResetInsufficient,
		outbox.UserEventAutomaticResetFailed, outbox.UserEventManualResetCompleted, outbox.UserEventManualResetRefunded:
		return p.Notifications.Traffic, nil
	case outbox.UserEventPaymentCredited, outbox.UserEventPaymentRefunded, outbox.UserEventMemberRefundCompleted, outbox.UserEventMemberRefundFailed:
		return p.Notifications.Money, nil
	case outbox.UserEventGroupReward, outbox.AffiliateSuccessKind, outbox.AffiliateTierUpgradeKind:
		return p.Notifications.Activity, nil
	case outbox.UserEventAdminExtension, outbox.UserEventAdminUpdate, outbox.UserEventNodeCompensation, outbox.UserEventProvisionConflict, outbox.UserAbuseNotificationKind:
		return p.Notifications.Account, nil
	default:
		return false, fmt.Errorf("notification category is missing for %q", kind)
	}
}
