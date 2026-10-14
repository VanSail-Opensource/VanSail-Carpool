package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	"strconv"
)

func (a proxyProvider) ListNotificationUsers(ctx context.Context, cursor string, size int) ([]notifications.TrafficUser, *string, bool, error) {
	after := int64(0)
	var err error
	if cursor != "" {
		after, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			return nil, nil, false, err
		}
	}
	if size < 1 || size > 500 {
		size = 200
	}
	identities, err := a.store.XNodeIdentities(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	out := []notifications.TrafficUser{}
	for _, identity := range identities {
		if identity.ProxyUserID <= after || identity.UserID == "" || identity.CurrentPeriodID == "" {
			continue
		}
		period, err := a.store.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return nil, nil, false, err
		}
		out = append(out, notifications.TrafficUser{ID: identity.ProxyUserID, UsedBytes: period.UsedBytes, LimitBytes: period.LimitBytes, ResetStrategy: period.Strategy, LastTrafficResetAt: period.LastResetAt})
		if len(out) == size {
			next := strconv.FormatInt(identity.ProxyUserID, 10)
			return out, &next, true, nil
		}
	}
	return out, nil, false, nil
}
