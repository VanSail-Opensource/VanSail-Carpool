package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
)

type notificationUserPage struct {
	users []remnawave.User
	next  *string
	more  bool
}

func (a remnaAdapter) ListNotificationUsers(ctx context.Context, cursor string, size int) ([]notifications.TrafficUser, *string, bool, error) {
	page, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (notificationUserPage, error) {
		users, next, more, callErr := client.ListUsersStream(callCtx, cursor, size)
		return notificationUserPage{users: users, next: next, more: more}, callErr
	})
	if err != nil {
		return nil, nil, false, err
	}
	users := make([]notifications.TrafficUser, 0, len(page.users))
	for _, user := range page.users {
		users = append(users, notifications.TrafficUser{
			ID: user.ID, UsedBytes: user.UserTraffic.UsedTrafficBytes, LimitBytes: user.TrafficLimitBytes,
			ResetStrategy: string(user.TrafficLimitStrategy), LastTrafficResetAt: user.LastTrafficResetAt,
		})
	}
	return users, page.next, page.more, nil
}

var _ notifications.TrafficRemote = remnaAdapter{}
