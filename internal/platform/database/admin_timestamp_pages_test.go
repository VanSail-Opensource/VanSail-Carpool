package database

import (
	"context"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestAdminPagesPreserveChronologicalNanosecondsAndStableTies(t *testing.T) {
	t.Parallel()
	f := newAdminTimestampFixture(t)
	ctx := context.Background()
	t.Run("payments", func(t *testing.T) {
		assertAdminTimestampPages(t, f.recordIDs, func(cursor string) ([]model.PaymentOrder, *string, error) {
			return f.store.ListAdminPaymentOrdersPage(ctx, cursor, "exact-page", "refunded", 1)
		}, func(item model.PaymentOrder) string { return item.ID })
	})
	t.Run("refunds", func(t *testing.T) {
		assertAdminTimestampPages(t, f.recordIDs, func(cursor string) ([]model.Refund, *string, error) {
			return f.store.ListAdminRefundsPage(ctx, cursor, "exact-page", "completed", 1)
		}, func(item model.Refund) string { return item.ID })
	})
	t.Run("jobs", func(t *testing.T) {
		assertAdminTimestampPages(t, f.recordIDs, func(cursor string) ([]model.OutboxJob, *string, error) {
			return f.store.ListAdminOutboxJobsPage(ctx, cursor, "exact_page", "failed", 1)
		}, func(item model.OutboxJob) string { return item.ID })
	})
	t.Run("users", func(t *testing.T) {
		assertAdminTimestampPages(t, f.userIDs, func(cursor string) ([]AdminUserRecord, *string, error) {
			return f.store.ListAdminUsersPage(ctx, cursor, AdminUserSearchFilter{Search: "Exact page"}, 1)
		}, func(item AdminUserRecord) string { return item.User.ID })
	})
	t.Run("entitlements", func(t *testing.T) {
		assertAdminTimestampPages(t, f.recordIDs, func(cursor string) ([]model.Purchase, *string, error) {
			return f.store.ListAdminPurchasesPage(ctx, cursor, "exact-page", "", 1)
		}, func(item model.Purchase) string { return item.ID })
	})
}

func TestAdminRecentRecordsSelectNewestExactInstant(t *testing.T) {
	t.Parallel()
	f := newAdminTimestampFixture(t)
	ctx := context.Background()
	refunds, err := f.store.ListRefundsForUser(ctx, f.owner, 2)
	assertAdminTimestampIDs(t, refunds, f.recordIDs[:2], err, func(item model.Refund) string { return item.ID })
	operations, err := f.store.ListAdminOperationsForUser(ctx, f.owner, 2)
	assertAdminTimestampIDs(t, operations, f.recordIDs[:2], err, func(item model.OperationReceipt) string { return item.ID })
	operations, err = f.store.ListProviderOperationsForOwner(ctx, f.owner, 2)
	assertAdminTimestampIDs(t, operations, f.recordIDs[:2], err, func(item model.OperationReceipt) string { return item.ID })
	audits, err := f.store.ListAuditEvents(ctx, 2)
	assertAdminTimestampIDs(t, audits, f.recordIDs[:2], err, func(item model.AuditEvent) string { return item.ID })
	backups, err := f.store.ListBackupRuns(ctx, 2)
	assertAdminTimestampIDs(t, backups, f.recordIDs[:2], err, func(item model.BackupRun) string { return item.ID })
	users, err := f.store.ListUsers(ctx, 2)
	assertAdminTimestampIDs(t, users, f.userIDs[:2], err, func(item model.User) string { return item.ID })
}

func assertAdminTimestampPages[T any](t *testing.T, want []string, read func(string) ([]T, *string, error), id func(T) string) {
	t.Helper()
	cursor := ""
	var got []string
	for index := 0; index <= len(want); index++ {
		items, next, err := read(cursor)
		if err != nil || len(items) != 1 {
			t.Fatalf("page %d = %v, %v", index, items, err)
		}
		got = append(got, id(items[0]))
		if next == nil {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("chronological page IDs = %v, want %v", got, want)
			}
			return
		}
		if *next == cursor {
			t.Fatal("pagination cursor did not advance")
		}
		cursor = *next
	}
	t.Fatal("pagination repeated records instead of reaching the end")
}

func assertAdminTimestampIDs[T any](t *testing.T, items []T, want []string, err error, id func(T) string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, id(item))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("newest record IDs = %v, want %v", got, want)
	}
}
