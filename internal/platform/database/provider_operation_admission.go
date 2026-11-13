package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// These production kinds are fresh administrator commands. Member commands and
// already committed recovery retain their own domain admission. Replays are
// returned before this check, so later demotion cannot change accepted intent.
func providerOperationAdminActorTx(ctx context.Context, tx *sql.Tx, input providerops.CreateInput) error {
	switch input.Kind {
	case providerops.KindAdminEntitlementEdit, providerops.KindAdminEntitlementRefund,
		providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension,
		providerops.KindAdminTemporaryBan, providerops.KindAdminTemporaryUnban,
		providerops.KindAdminMaintenance, providerops.KindPaymentRefund, providerops.KindOutboxRetry,
		"xnode_command", "xnode_firewall_exemption":
		return catalogAdminActorTx(ctx, tx, input.ActorUserID)
	default:
		return nil
	}
}
