package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func newAccountCleanup(store *database.Store, client *queuedTelegram, token string, admins []int64, worker *outbox.Worker) (*accounts.AccountCleanupService, error) {
	prefix, _, found := strings.Cut(token, ":")
	botID, err := strconv.ParseInt(prefix, 10, 64)
	if !found || err != nil {
		return nil, errors.New("account cleanup bot identity is unavailable")
	}
	service, err := accounts.NewAccountCleanup(store, client, botID, admins)
	if err != nil {
		return nil, err
	}
	if err = worker.Register(accounts.AccountCleanupJob, service); err != nil {
		return nil, err
	}
	return service, nil
}

func (a *Application) enqueueAccountCleanup(ctx context.Context, now time.Time) {
	if a.accountCleanup == nil {
		return
	}
	if err := a.accountCleanup.Schedule(ctx, now.UTC()); err != nil {
		a.logger.Warn("account cleanup admission deferred", "error_code", "ACCOUNT_CLEANUP_SCHEDULE_PENDING")
	}
}

func (a *Application) verifyAccountCleanupBot(ctx context.Context) {
	if a.accountCleanup == nil {
		return
	}
	if err := a.accountCleanup.VerifyBot(ctx); err != nil {
		a.logger.Warn("account cleanup bot verification paused", "error_code", "ACCOUNT_CLEANUP_BOT_UNVERIFIED")
	}
}
