package app

import (
	"context"
)

func (a remnaAdapter) SetAdminUserDisabled(ctx context.Context, remoteID string, disabled bool) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		if disabled {
			return client.DisableUser(callCtx, userID)
		}
		return client.EnableUser(callCtx, userID)
	})
}
