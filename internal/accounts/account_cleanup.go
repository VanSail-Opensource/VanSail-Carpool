package accounts

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const AccountCleanupJob = "account_cleanup_probe"

type AccountCleanupRepository interface {
	SetAccountCleanupBot(context.Context, int64, time.Time) (model.AccountCleanupHealth, error)
	EnqueueAccountCleanupChecks(context.Context, time.Time, int) error
	AccountCleanupProbe(context.Context, string, int64, string) (*model.AccountCleanupProbe, error)
	CompleteAccountCleanupProbe(context.Context, model.AccountCleanupProbe, model.AccountCleanupObservation, time.Time) (model.AccountCleanupCompletion, error)
}

type AccountCleanupTelegram interface {
	SendChatAction(context.Context, int64) (bool, error)
	GetChat(context.Context, int64) (telegram.ChatInfo, error)
	GetMe(context.Context) (telegram.User, error)
}

// AccountCleanupService coordinates the existing outbox and Telegram queue. It
// never authorizes deletion itself: durable evidence/admin gates own admission.
type AccountCleanupService struct {
	repository            AccountCleanupRepository
	telegram              AccountCleanupTelegram
	expectedBotID         int64
	protected             map[int64]bool
	probeBusy, verifyBusy atomic.Bool
	now                   func() time.Time
}

func NewAccountCleanup(repository AccountCleanupRepository, client AccountCleanupTelegram, botID int64, admins []int64) (*AccountCleanupService, error) {
	if repository == nil || client == nil || botID <= 0 || botID > 1<<52-1 {
		return nil, errors.New("invalid account cleanup dependencies")
	}
	service := &AccountCleanupService{repository: repository, telegram: client, expectedBotID: botID, protected: map[int64]bool{}, now: time.Now}
	for _, id := range admins {
		if id <= 0 || id > 1<<52-1 {
			return nil, errors.New("invalid protected administrator id")
		}
		service.protected[id] = true
	}
	return service, nil
}

func (s *AccountCleanupService) Schedule(ctx context.Context, at time.Time) error {
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.repository.EnqueueAccountCleanupChecks(bounded, at.UTC(), 64)
}
