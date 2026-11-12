package app

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

const starsHistoryPageSize = 100

type starsReconciliationRepository interface {
	EnsureTelegramStarReconciliation(context.Context, int64, time.Time) error
	TelegramStarReconciliationCursor(context.Context, int64) (database.TelegramStarCursor, error)
	AdvanceTelegramStarReconciliation(context.Context, database.TelegramStarCursor, int64, string, time.Time, time.Time) error
	RecordTelegramStarRefundReview(context.Context, database.TelegramStarRefundReview, time.Time) error
	RefundPayment(context.Context, *string, string, string, time.Time) (model.PaymentOrder, error)
	ResolvePaymentRefundOperation(context.Context, string, string, time.Time) error
}

type starsReconciliationBilling interface {
	Settle(context.Context, billing.ProviderEvent) (model.PaymentOrder, bool, error)
	ValidateEvent(context.Context, billing.ProviderEvent) (model.PaymentOrder, error)
}

type starsReconciler struct {
	botID      int64
	provider   *queuedTelegram
	repository starsReconciliationRepository
	billing    starsReconciliationBilling
	now        func() time.Time
}

func newStarsReconciler(repository starsReconciliationRepository, service starsReconciliationBilling,
	provider *queuedTelegram, token string, worker *outbox.Worker) (*starsReconciler, error) {
	prefix, _, found := strings.Cut(token, ":")
	botID, err := strconv.ParseInt(prefix, 10, 64)
	if !found || err != nil || botID <= 0 {
		return nil, errors.New("Stars reconciliation bot identity is unavailable")
	}
	reconciler := &starsReconciler{botID: botID, provider: provider, repository: repository, billing: service, now: time.Now}
	if err := worker.Register(jobpayload.StarsReconciliationKind, reconciler); err != nil {
		return nil, err
	}
	return reconciler, nil
}

func (a *Application) enqueueStarsReconciliation(ctx context.Context, now time.Time) {
	if a.stars == nil {
		return
	}
	if err := a.stars.repository.EnsureTelegramStarReconciliation(ctx, a.stars.botID, now); err != nil {
		a.logger.Error("resume Stars history reconciliation", "error", err)
	}
}

// HandleOutbox processes at most one page, retaining the boundary on failure.
// Telegram returns ascending history. Overlap validates numeric offsets before
// new records are applied; changed or missing anchors rewind without skipping.
func (w *starsReconciler) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	var page jobpayload.StarsReconciliationPage
	if job.Kind != jobpayload.StarsReconciliationKind || json.Unmarshal([]byte(job.Payload), &page) != nil || page.BotID <= 0 || page.Generation <= 0 {
		return errors.New("Stars reconciliation page is invalid")
	}
	if page.BotID != w.botID {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cursor, err := w.repository.TelegramStarReconciliationCursor(ctx, page.BotID)
	if err != nil || cursor.Generation != page.Generation {
		return err
	}
	offset := max(int64(0), cursor.NextOffset-1)
	transactions, err := w.provider.GetStarTransactions(ctx, int(offset), starsHistoryPageSize)
	if err != nil {
		return err
	}
	if len(transactions) > starsHistoryPageSize {
		return errors.New("Stars history page exceeds its requested limit")
	}
	now := w.now().UTC()
	if cursor.NextOffset > 0 && (len(transactions) == 0 || starTransactionHash(transactions[0]) != cursor.Anchor) {
		return w.repository.AdvanceTelegramStarReconciliation(ctx, cursor, 0, "", now.Add(time.Second), now)
	}
	start := 0
	if cursor.NextOffset > 0 {
		start = 1
	}
	for _, transaction := range transactions[start:] {
		if err := w.applyTransaction(ctx, transaction); err != nil {
			return err
		}
	}
	now = w.now().UTC()
	nextOffset, anchor := offset+int64(len(transactions)), cursor.Anchor
	if nextOffset > math.MaxInt32 {
		return errors.New("Stars history exceeds the provider offset range")
	}
	if len(transactions) > 0 {
		anchor = starTransactionHash(transactions[len(transactions)-1])
	}
	next := now.Add(time.Second)
	if len(transactions) < starsHistoryPageSize {
		next = now.Add(5 * time.Minute)
	}
	return w.repository.AdvanceTelegramStarReconciliation(ctx, cursor, nextOffset, anchor, next, now)
}
