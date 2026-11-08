package accounts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type purgeGuardFixture struct {
	purgeFixture
	guards, denyAt, fences int
	reason, dependency     string
}

func (f *purgeGuardFixture) AccountCleanupAdmissionValid(context.Context, string, int64, time.Time) (bool, error) {
	f.guards++
	return f.guards != f.denyAt, nil
}
func (f *purgeGuardFixture) AccountCleanupAdmissionReason(context.Context, string, int64, time.Time) (string, error) {
	return f.reason, nil
}
func (f *purgeGuardFixture) FenceAccountPurge(context.Context, string, int64, time.Time) error {
	f.fences++
	return nil
}
func (f *purgeGuardFixture) AccountPurgeDependency(context.Context, string, int64, time.Time) (string, error) {
	return f.dependency, nil
}

func TestPurgeRechecksAdmissionBeforeEveryDestructivePhase(t *testing.T) {
	for phase := 1; phase <= 4; phase++ {
		t.Run([]string{"entry", "remote", "native", "final"}[phase-1], func(t *testing.T) {
			f := &purgeGuardFixture{purgeFixture: purgeFixture{target: &model.AccountPurge{
				UserID: "member", Generation: 2, ThroughGeneration: 1, RemoteEmbyID: "emby",
			}}, denyAt: phase, reason: "fresh_final_check_required"}
			remote := 0
			w := PurgeWorker{Repository: f, Native: f, DeleteEmby: func(context.Context, string) error { remote++; return nil }}
			err := w.HandleOutbox(context.Background(), model.OutboxJob{ID: "canonical", Payload: `{"userId":"member","generation":2}`})
			var wait *outbox.WaitError
			if !errors.As(err, &wait) || wait.Reason != "account_purge_fresh_final_check_required" || f.completed != 0 {
				t.Fatal("expired proof did not pause the exact phase", f, err)
			}
			wantRemote, wantNative, wantFence := 0, 0, 0
			if phase > 1 {
				wantFence = 1
			}
			if phase > 2 {
				wantRemote = 1
			}
			if phase > 3 {
				wantNative = 1
			}
			if remote != wantRemote || f.closed != wantNative || f.fences != wantFence {
				t.Fatal("cleanup crossed an invalid admission", remote, f.closed, f.fences)
			}
		})
	}
}

func TestPurgeFencesClaimedIntentBeforeWaitingForBusyMutation(t *testing.T) {
	f := &purgeGuardFixture{purgeFixture: purgeFixture{target: &model.AccountPurge{UserID: "member", Generation: 1, Busy: true}}}
	w := PurgeWorker{Repository: f, Native: f}
	err := w.HandleOutbox(context.Background(), model.OutboxJob{ID: "canonical", Payload: `{"userId":"member","generation":1}`})
	var wait *outbox.WaitError
	if !errors.As(err, &wait) || wait.Reason != "account_purge_mutations_pending" || f.fences != 1 || f.closed != 0 || f.completed != 0 {
		t.Fatal("busy claim left an unattempted mutation admissible", f, err)
	}
}

func TestPurgePreservesExplicitDependencyAndProtectedAccount(t *testing.T) {
	for _, protected := range []bool{false, true} {
		f := &purgeGuardFixture{purgeFixture: purgeFixture{target: &model.AccountPurge{UserID: "member", Generation: 1}}, dependency: "account_purge_payment_outcome_pending"}
		if protected {
			f.denyAt, f.reason = 1, "administrator_protected"
		}
		w := PurgeWorker{Repository: f, Native: f}
		err := w.HandleOutbox(context.Background(), model.OutboxJob{ID: "canonical", Payload: `{"userId":"member","generation":1}`})
		var wait *outbox.WaitError
		var terminal *outbox.TerminalError
		if protected && (!errors.As(err, &terminal) || f.fences != 0) || !protected && (!errors.As(err, &wait) || wait.Reason != f.dependency) {
			t.Fatal("cleanup lost its safety reason", f, err)
		}
		if f.closed != 0 || f.completed != 0 {
			t.Fatal("blocked cleanup changed native or local state", f)
		}
	}
}
