package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
)

func (a proxyProvider) ListNotificationUsers(ctx context.Context, cursor string, size int) ([]notifications.TrafficUser, *string, bool, error) {
	after := int64(0)
	var err error
	if cursor != "" {
		after, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			return nil, nil, false, err
		}
		if after < 0 {
			return nil, nil, false, errors.New("invalid native traffic cursor")
		}
	}
	if size < 1 || size > 1000 {
		size = 200
	}
	users, err := a.store.XNodeNotificationUsers(ctx, after, size+1)
	if err != nil {
		return nil, nil, false, err
	}
	if len(users) <= size {
		return users, nil, false, nil
	}
	users = users[:size]
	next := strconv.FormatInt(users[len(users)-1].ID, 10)
	return users, &next, true, nil
}
