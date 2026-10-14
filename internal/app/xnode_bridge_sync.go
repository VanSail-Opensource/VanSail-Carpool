package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
	"time"
)

// Stream once per interval through the existing queue, then sync linked accounts
// only. Unmapped remote identities are neither persisted nor authorized.
func (a proxyProvider) syncBridgeAccounts(ctx context.Context) error {
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge {
		return err
	}
	identities, err := a.store.XNodeIdentities(ctx)
	if err != nil {
		return err
	}
	linked := map[int64]xnode.Identity{}
	for _, identity := range identities {
		if identity.LegacyUserID != "" {
			id, err := strconv.ParseInt(identity.LegacyUserID, 10, 64)
			if err != nil {
				return err
			}
			linked[id] = identity
		}
	}
	if len(linked) == 0 {
		return nil
	}
	cursor := ""
	var firstError error
	for {
		page, err := remnaCall(ctx, a.remnaAdapter, func(ctx context.Context, client remnaClient) (notificationUserPage, error) {
			users, next, more, err := client.ListUsersStream(ctx, cursor, 200)
			return notificationUserPage{users: users, next: next, more: more}, err
		})
		if err != nil {
			return errors.Join(firstError, err)
		}
		for _, user := range page.users {
			identity, ok := linked[user.ID]
			if !ok {
				continue
			}
			err := a.native.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: user.UserTraffic.UsedTrafficBytes, LifetimeBytes: user.UserTraffic.LifetimeUsedTrafficBytes, LastResetAt: user.LastTrafficResetAt, SyncedAt: time.Now().UTC()})
			if err == nil && identity.UserID != "" {
				err = a.applyBridgePolicy(ctx, identity, user.UserTraffic.UsedTrafficBytes, &user)
			}
			if err != nil && firstError == nil {
				firstError = err
			}
		}
		if !page.more {
			return firstError
		}
		if page.next == nil || *page.next == cursor {
			return xnode.ErrInvalid
		}
		cursor = *page.next
	}
}
