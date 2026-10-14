package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
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

func (a remnaAdapter) VerifyAdminRemoteUser(ctx context.Context, remoteID string) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	_, err = remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(callCtx, userID)
	})
	return err
}
