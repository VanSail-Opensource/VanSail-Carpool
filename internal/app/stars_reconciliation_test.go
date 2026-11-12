package app

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestStarsHistoryResumesBeyondFirstPageAndPollsOnlyBoundary(t *testing.T) {
	ctx, now := context.Background(), time.Now().UTC()
	history := starsIncomingHistory(205)
	var offsets []int
	var historyMu sync.Mutex
	provider := starsHistoryTelegramForTest(t, func(offset, limit int) ([]telegram.StarTransaction, error) {
		historyMu.Lock()
		defer historyMu.Unlock()
		offsets = append(offsets, offset)
		return history[min(offset, len(history)):min(offset+limit, len(history))], nil
	})
	repository := &starsMemoryRepository{cursor: database.TelegramStarCursor{BotID: 123, Generation: 1}}
	service := &starsMemoryBilling{settled: make(map[string]bool)}
	newWorker := func() *starsReconciler {
		return &starsReconciler{botID: 123, provider: provider, repository: repository, billing: service, now: func() time.Time { return now }}
	}
	for i := range 3 {
		// Reconstruct the worker for every page; progress belongs to persistence.
		if err := newWorker().HandleOutbox(ctx, starsPageJob(t, repository.cursor)); err != nil {
			t.Fatal(err)
		}
		if i == 0 && (len(service.settled) != 100 || !repository.cursor.AvailableAt.Equal(now.Add(time.Second))) {
			t.Fatalf("unbounded first page: settled=%d cursor=%+v", len(service.settled), repository.cursor)
		}
	}
	if len(service.settled) != 205 || repository.cursor.NextOffset != 205 || !repository.cursor.AvailableAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("history recovery = settled:%d cursor:%+v", len(service.settled), repository.cursor)
	}
	if err := newWorker().HandleOutbox(ctx, starsPageJob(t, repository.cursor)); err != nil {
		t.Fatal(err)
	}
	historyMu.Lock()
	caughtUpOffsets := append([]int(nil), offsets...)
	history = starsIncomingHistory(207)
	historyMu.Unlock()
	if service.calls != 205 || !reflect.DeepEqual(caughtUpOffsets, []int{0, 99, 198, 204}) {
		t.Fatalf("caught-up poll rescanned history: calls=%d offsets=%v", service.calls, caughtUpOffsets)
	}
	if err := newWorker().HandleOutbox(ctx, starsPageJob(t, repository.cursor)); err != nil {
		t.Fatal(err)
	}
	historyMu.Lock()
	lastOffset := offsets[len(offsets)-1]
	historyMu.Unlock()
	if len(service.settled) != 207 || lastOffset != 204 {
		t.Fatalf("new tail not recovered: settled=%d offset=%d", len(service.settled), lastOffset)
	}
}

func TestStarsHistoryRetriesPartialPageWithoutAdvancing(t *testing.T) {
	for _, failure := range []string{"settlement", "cursor commit"} {
		t.Run(failure, func(t *testing.T) {
			history := starsIncomingHistory(100)
			provider := starsHistoryTelegramForTest(t, func(int, int) ([]telegram.StarTransaction, error) { return history, nil })
			repository := &starsMemoryRepository{cursor: database.TelegramStarCursor{BotID: 123, Generation: 1}, failAdvance: failure == "cursor commit"}
			service := &starsMemoryBilling{settled: make(map[string]bool)}
			if failure == "settlement" {
				service.failOnceOrder = "order-20"
			}
			worker := &starsReconciler{botID: 123, provider: provider, repository: repository, billing: service, now: time.Now}
			job := starsPageJob(t, repository.cursor)
			if err := worker.HandleOutbox(context.Background(), job); err == nil || repository.cursor.NextOffset != 0 || repository.cursor.Generation != 1 {
				t.Fatalf("failed page advanced cursor: %+v, %v", repository.cursor, err)
			}
			if err := worker.HandleOutbox(context.Background(), job); err != nil || len(service.settled) != 100 || repository.cursor.NextOffset != 100 {
				t.Fatalf("retry lost/duplicated settlement: count=%d cursor=%+v error=%v", len(service.settled), repository.cursor, err)
			}
			calls := service.calls
			if err := worker.HandleOutbox(context.Background(), job); err != nil || service.calls != calls {
				t.Fatalf("stale generation attempted provider/settlement: %d -> %d, %v", calls, service.calls, err)
			}
		})
	}
}

func TestStarsHistoryRewindsShiftedOrMissingAnchor(t *testing.T) {
	for _, missing := range []bool{false, true} {
		history := starsIncomingHistory(101)
		cursor := database.TelegramStarCursor{BotID: 123, Generation: 4, NextOffset: 100, Anchor: starTransactionHash(history[99])}
		if missing {
			history = nil
		} else {
			history = append([]telegram.StarTransaction{{ID: "new-older-history-record"}}, history...)
		}
		provider := starsHistoryTelegramForTest(t, func(offset, limit int) ([]telegram.StarTransaction, error) {
			return history[min(offset, len(history)):min(offset+limit, len(history))], nil
		})
		repository := &starsMemoryRepository{cursor: cursor}
		service := &starsMemoryBilling{settled: make(map[string]bool)}
		worker := &starsReconciler{botID: 123, provider: provider, repository: repository, billing: service, now: time.Now}
		if err := worker.HandleOutbox(context.Background(), starsPageJob(t, cursor)); err != nil || service.calls != 0 || repository.cursor.NextOffset != 0 || repository.cursor.Anchor != "" {
			t.Fatalf("unsafe shifted boundary: missing=%t cursor=%+v calls=%d error=%v", missing, repository.cursor, service.calls, err)
		}
	}
}
