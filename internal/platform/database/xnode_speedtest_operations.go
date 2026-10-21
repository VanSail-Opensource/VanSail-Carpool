package database

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) ReplayXNodeRouteSpeedtest(ctx context.Context, user, key, fingerprint string) (model.OperationReceipt, bool, error) {
	op, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` WHERE actor_user_id=? AND kind=? AND idempotency_key=?`, user, xnode.RouteSpeedtestOperation, key))
	if errors.Is(err, ErrNotFound) {
		return model.OperationReceipt{}, false, nil
	}
	if err != nil {
		return model.OperationReceipt{}, false, err
	}
	if op.OwnerUserID != user || op.RequestFingerprint != fingerprint {
		return model.OperationReceipt{}, false, xnode.ErrConflict
	}
	return op.Receipt, true, nil
}
