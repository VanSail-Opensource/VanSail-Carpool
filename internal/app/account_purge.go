package app

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func registerAccountPurge(worker *outbox.Worker, store *database.Store, native *xnode.Service, remote embyAdapter) error {
	return worker.Register(accounts.PurgeJob, &accounts.PurgeWorker{Repository: store, Native: native, DeleteEmby: remote.deleteAccount})
}

func (a embyAdapter) deleteAccount(ctx context.Context, id string) error {
	ctx = upstreamqueue.WithKey(ctx, "user:"+id)
	err := embyExecute(ctx, a, func(callCtx context.Context, client embyClient) error {
		deleter, ok := client.(interface {
			DeleteUser(context.Context, string) error
		})
		if !ok {
			return errors.New("Emby account deletion is unavailable")
		}
		return deleter.DeleteUser(callCtx, id)
	})
	if a.IsNotFound(err) {
		return nil
	}
	return err
}
