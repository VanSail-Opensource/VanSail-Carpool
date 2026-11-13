package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AdminUserSynchronization describes current native credential provisioning and
// historical deployment proof. It does not assert live agent health or access.
type AdminUserSynchronization struct {
	Status       string
	RemoteUserID *string
	LastError    *string
}

const adminUserSynchronizationColumns = `native.proxy_user_id,
	COALESCE(native.credentials_cipher<>'' AND native.subscription_cipher<>''
		AND octet_length(native.subscription_hash)=32 AND COALESCE(native.service_name,'')='',false),
	native.first_synced_at IS NOT NULL`

func adminUserSynchronizationFor(user model.User, nativeID sql.NullInt64, credentialsReady, applied bool) AdminUserSynchronization {
	state := AdminUserSynchronization{Status: "not_provisioned"}
	if user.AccountStatus == "active" && user.OnboardingState == "complete" && credentialsReady &&
		nativeID.Valid && nativeID.Int64 > 0 && nativeID.Int64 < 1<<53 {
		id := strconv.FormatInt(nativeID.Int64, 10)
		state.RemoteUserID = &id
		state.Status = "pending"
		if applied {
			state.Status = "synchronized"
		}
	}
	if user.RecoveryReason != "" {
		state.Status = "failed"
		state.LastError = &user.RecoveryReason
	}
	return state
}

// AdminUserSynchronization reads owner lifecycle and its main native identity in
// one snapshot. Legacy provider links never qualify native deployment evidence.
func (s *Store) AdminUserSynchronization(ctx context.Context, userID string) (AdminUserSynchronization, error) {
	var user model.User
	var nativeID sql.NullInt64
	var credentialsReady, applied bool
	err := s.db.QueryRowContext(ctx, `SELECT users.account_status,users.onboarding_state,COALESCE(users.recovery_reason,''),`+
		adminUserSynchronizationColumns+` FROM users
		LEFT JOIN xnode_identities native ON native.user_id=users.id WHERE users.id=$1`, userID).
		Scan(&user.AccountStatus, &user.OnboardingState, &user.RecoveryReason, &nativeID, &credentialsReady, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminUserSynchronization{}, ErrNotFound
	}
	if err != nil {
		return AdminUserSynchronization{}, err
	}
	return adminUserSynchronizationFor(user, nativeID, credentialsReady, applied), nil
}
