package preferences

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"testing"
)

func TestAllNotificationKindsHaveExactlyOneCategory(t *testing.T) {
	groups := [][]string{
		{outbox.UserEventExpiration, outbox.UserEventExpiryReminder, outbox.UserEventQueuedActivation, outbox.UserEventPurchaseActivation, outbox.UserEventAutoRenewal, outbox.UserEventPurchaseQueued, outbox.UserEventRenewalScheduled, outbox.UserEventAddonActivated, outbox.UserEventQueuedCancellation, outbox.UserEventAutoRenewalFailed},
		{outbox.UserEventTrafficThreshold, outbox.UserEventAutomaticReset, outbox.UserEventAutomaticResetInsufficient, outbox.UserEventAutomaticResetFailed, outbox.UserEventManualResetCompleted, outbox.UserEventManualResetRefunded},
		{outbox.UserEventPaymentCredited, outbox.UserEventPaymentRefunded, outbox.UserEventMemberRefundCompleted, outbox.UserEventMemberRefundFailed},
		{outbox.UserEventGroupReward, outbox.AffiliateSuccessKind, outbox.AffiliateTierUpgradeKind},
		{outbox.UserEventAdminExtension, outbox.UserEventAdminUpdate, outbox.UserEventNodeCompensation, outbox.UserEventProvisionConflict, outbox.UserAbuseNotificationKind},
	}
	for category, kinds := range groups {
		for _, kind := range kinds {
			t.Run(kind, func(t *testing.T) {
				for enabled := range groups {
					p := Preferences{Notifications: Notifications{enabled == 0, enabled == 1, enabled == 2, enabled == 3, enabled == 4}}
					allowed, err := p.Allows(kind)
					if err != nil || allowed != (category == enabled) {
						t.Fatalf("category=%d enabled=%d allowed=%t error=%v", category, enabled, allowed, err)
					}
				}
			})
		}
	}
	if _, err := Defaults().Allows("unknown"); err == nil {
		t.Fatal("unknown event bypassed category resolution")
	}
}
