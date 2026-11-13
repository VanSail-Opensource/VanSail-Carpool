package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestEmbyConnectionCASAndSnapshotStayAtomic(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 498010)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	initial, err := store.EmbyConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pair := func(id int) EmbyConnectionRecord {
		return EmbyConnectionRecord{
			BaseURL: Setting{Key: EmbyURLSetting, Value: fmt.Sprintf("url-%d", id), Encrypted: true},
			Token:   Setting{Key: EmbyTokenSetting, Value: fmt.Sprintf("token-%d", id), Encrypted: true},
		}
	}
	var group sync.WaitGroup
	errorsCh := make(chan error, 3)
	for i := range 2 {
		group.Go(func() { errorsCh <- store.SaveEmbyConnection(ctx, initial.Version(), pair(i), user.ID) })
	}
	group.Wait()
	close(errorsCh)
	won, conflicted := 0, 0
	for err := range errorsCh {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrConflict):
			conflicted++
		default:
			t.Fatal(err)
		}
	}
	if won != 1 || conflicted != 1 {
		t.Fatal("competing first-time setup did not CAS", won, conflicted)
	}
	stop := make(chan struct{})
	failure := make(chan string, 1)
	group.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			read, err := store.EmbyConnection(ctx)
			if err != nil || strings.TrimPrefix(read.BaseURL.Value, "url-") != strings.TrimPrefix(read.Token.Value, "token-") {
				failure <- fmt.Sprintf("mixed connection snapshot: %v", err)
				return
			}
		}
	})
	for i := 2; i < 25; i++ {
		current, err := store.EmbyConnection(ctx)
		if err != nil {
			t.Error(err)
			break
		}
		if err := store.SaveEmbyConnection(ctx, current.Version(), pair(i), user.ID); err != nil {
			t.Error(err)
			break
		}
	}
	close(stop)
	group.Wait()
	select {
	case err := <-failure:
		t.Fatal(err)
	default:
	}
}

func TestEmbyConnectionRequiresAdminAndAtomicRedactedAudit(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 498011)
	initial, err := store.EmbyConnection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next := EmbyConnectionRecord{
		BaseURL: Setting{Key: EmbyURLSetting, Value: "encrypted-private-endpoint", Encrypted: true},
		Token:   Setting{Key: EmbyTokenSetting, Value: "encrypted-private-token", Encrypted: true},
	}
	for _, id := range []string{"", actor.ID} {
		if err := store.SaveEmbyConnection(ctx, initial.Version(), next, id); !errors.Is(err, ErrConflict) {
			t.Fatalf("non-admin connection update = %v", err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE FUNCTION reject_emby_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`,
		`CREATE TRIGGER reject_emby_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.action='emby.connection.update') EXECUTE FUNCTION reject_emby_audit()`,
	} {
		if _, err := store.DB().ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveEmbyConnection(ctx, initial.Version(), next, actor.ID); err == nil {
		t.Fatal("connection committed without its audit")
	}
	unchanged, err := store.EmbyConnection(ctx)
	if err != nil || unchanged.Version() != initial.Version() {
		t.Fatalf("audit failure left changed connection: %+v, %v", unchanged, err)
	}
	if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER reject_emby_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEmbyConnection(ctx, initial.Version(), next, actor.ID); err != nil {
		t.Fatal(err)
	}
	events, err := store.ListAuditEvents(ctx, 10)
	if err != nil || len(events) != 1 || events[0].Action != "emby.connection.update" || events[0].ActorUserID == nil || *events[0].ActorUserID != actor.ID {
		t.Fatalf("committed connection audit = %+v, %v", events, err)
	}
	if strings.Contains(events[0].Detail, next.BaseURL.Value) || strings.Contains(events[0].Detail, next.Token.Value) {
		t.Fatal("connection audit retained secret material")
	}
}
