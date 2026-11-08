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
