package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestRetiredDetectorReleasePreservesManualRestrictionAndRetries(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name                  string
		pending, manual, fail bool
		calls, completed      int
	}{
		{name: "completed historical ban", pending: true, calls: 1, completed: 1},
		{name: "manual ban wins", pending: true, manual: true, completed: 1},
		{name: "unacknowledged or replay", pending: false},
		{name: "provider retry preserves pending", pending: true, fail: true, calls: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repo := &detectorReleaseRepoStub{pending: scenario.pending, manual: scenario.manual}
			provider := &detectorReleaseProviderStub{fail: scenario.fail}
			err := handleRetiredDetectorRelease(context.Background(), model.OutboxJob{Payload: `{"userId":"member"}`}, repo, provider)
			if (err != nil) != scenario.fail || provider.calls != scenario.calls || repo.completed != scenario.completed {
				t.Fatalf("err=%v calls=%d completed=%d", err, provider.calls, repo.completed)
			}
		})
	}
}

type detectorReleaseRepoStub struct {
	pending, manual bool
	completed       int
}

func (r *detectorReleaseRepoStub) RetiredDetectorReleaseUser(context.Context, string) (model.User, bool, error) {
	return model.User{ID: "member"}, r.pending, nil
}
func (r *detectorReleaseRepoStub) ActiveAdminTemporaryBan(context.Context, string) (*database.AdminTemporaryBan, error) {
	if r.manual {
		return &database.AdminTemporaryBan{UserID: "member"}, nil
	}
	return nil, nil
}
func (r *detectorReleaseRepoStub) CompleteRetiredDetectorRelease(context.Context, string, time.Time) error {
	r.completed++
	return nil
}

type detectorReleaseProviderStub struct {
	calls int
	fail  bool
}

func (p *detectorReleaseProviderStub) releaseRetiredDetector(context.Context, model.User) error {
	p.calls++
	if p.fail {
		return errors.New("provider temporarily unavailable")
	}
	return nil
}
