package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

var ErrPaymentProfileInUse = fmt.Errorf("%w: retained payment evidence requires a replacement profile", ErrConflict)

// Expired and cancelled orders can still receive authenticated late payments.
// Retain the provider identity and key for every retained order, including old
// two-part method identifiers whose account was not pinned at creation.
func paymentProfileReferencedTx(ctx context.Context, tx *sql.Tx, id, provider string) (bool, error) {
	prefix := provider + ":" + id + ":"
	var referenced bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_orders
		WHERE provider=$1 AND (substr(method_id,1,length($2))=$2 OR split_part(method_id,':',3)=''))`, provider, prefix).Scan(&referenced)
	if err != nil {
		return false, fmt.Errorf("check retained payment profile evidence: %w", err)
	}
	return referenced, nil
}

// Order admission shares the profile lock with identity edits/deletion. The
// serializable transaction also protects the legacy provider-wide predicate.
func lockPaymentOrderProfileTx(ctx context.Context, tx *sql.Tx, order model.PaymentOrder) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM payment_profiles WHERE provider=$1 AND
		(split_part($2,':',3)='' OR substr($2,1,length(provider||':'||id||':'))=provider||':'||id||':') ORDER BY id FOR SHARE`, order.Provider, order.MethodID)
	if err != nil {
		return fmt.Errorf("lock payment order account: %w", err)
	}
	defer rows.Close()
	matched := false
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		matched = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !matched && len(strings.Split(order.MethodID, ":")) == 3 {
		return ErrNotFound
	}
	return nil
}

func paymentProfileActorTx(ctx context.Context, tx *sql.Tx, actorID string) error {
	if actorID == "" {
		return nil // Bootstrap and automatic readiness changes use a system actor.
	}
	return catalogAdminActorTx(ctx, tx, actorID)
}

func paymentProfileAuditTx(ctx context.Context, tx *sql.Tx, actorID, action, profileID string, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	return insertAuditTx(ctx, tx, id, actor, action, "payment_profile", profileID, "{}", now)
}
