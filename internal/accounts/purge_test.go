package accounts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type purgeFixture struct {
	target            *model.AccountPurge
	closeErr          error
	completed, closed int
}

func (f *purgeFixture) AccountPurge(context.Context, string, int64) (*model.AccountPurge, error) {
	return f.target, nil
}
func (f *purgeFixture) CompleteAccountPurge(context.Context, string, int64, time.Time) error {
	f.completed++
	f.target = nil
	return nil
}
func (f *purgeFixture) CloseAccountPeriods(context.Context, string) error {
	f.closed++
	return f.closeErr
}

func TestPurgeWorkerWaitsForReliableCleanupBeforeCompleting(t *testing.T) {
	for _, test := range []struct {
		name                                    string
		busy, uncertain, remoteFail, nativeFail bool
	}{
		{name: "success"}, {name: "earlier mutation", busy: true}, {name: "uncertain creation", uncertain: true},
		{name: "remote failure", remoteFail: true}, {name: "native accounting pending", nativeFail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := &purgeFixture{target: &model.AccountPurge{UserID: "member", Generation: 1, RemoteEmbyID: "remote", Busy: test.busy, EmbyUncertain: test.uncertain}}
			if test.nativeFail {
				f.closeErr = errors.New("offline journal")
			}
			deletes := 0
			w := PurgeWorker{Repository: f, Native: f, DeleteEmby: func(context.Context, string) error {
				deletes++
				if test.remoteFail {
					return errors.New("remote unavailable")
				}
				return nil
			}}
			job := model.OutboxJob{Payload: `{"userId":"member","generation":1}`}
			err := w.HandleOutbox(context.Background(), job)
			if test.name == "success" {
				if err != nil || f.completed != 1 || f.closed != 1 || deletes != 1 {
					t.Fatal(f, err)
				}
				if err = w.HandleOutbox(context.Background(), job); err != nil || deletes != 1 {
					t.Fatal("cleanup replayed", err)
				}
			} else if err == nil || f.completed != 0 {
				t.Fatal("incomplete cleanup declared complete", f, err)
			}
			if (test.busy || test.uncertain) && deletes != 0 {
				t.Fatal("remote mutation overtook pending work")
			}
		})
	}
}
