package xnode

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type globalFenceRepository struct {
	Repository
	version  atomic.Int64
	boundary time.Time
	entered  chan struct{}
	release  chan struct{}
	once     sync.Once
}

func (r *globalFenceRepository) XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error) {
	return r.version.Load(), r.boundary, nil
}
func (r *globalFenceRepository) XNodeNodes(ctx context.Context) ([]Node, error) {
	r.once.Do(func() { close(r.entered) })
	select {
	case <-r.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestGlobalValidationRejectsSupersededConfigurationAndAccess(t *testing.T) {
	for _, change := range []string{"global", "automatic lease", "database epoch", "access expiry"} {
		t.Run(change, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			var clock atomic.Int64
			clock.Store(at.UnixNano())
			repo := &globalFenceRepository{entered: make(chan struct{}), release: make(chan struct{}), boundary: at.Add(time.Second)}
			repo.version.Store(7)
			var release sync.Once
			unblock := func() { release.Do(func() { close(repo.release) }) }
			defer unblock()
			service := NewService(repo, &globalCacheSettings{}, nil, nil, nil)
			service.now = func() time.Time { return time.Unix(0, clock.Load()).UTC() }
			before, err := service.Global(ctx)
			if err != nil {
				t.Fatal(err)
			}
			candidate := cloneGlobal(before)
			candidate.Telemetry.ReportSeconds = 10
			var validationErr error
			done := make(chan struct{})
			go func() {
				validationErr = service.SaveGlobal(ctx, "administrator", candidate)
				close(done)
			}()
			defer func() { cancel(); unblock(); <-done }()
			select {
			case <-repo.entered:
			case <-ctx.Done():
				t.Fatal("global validation did not start", ctx.Err())
			}
			write := make(chan error, 1)
			go func() {
				switch change {
				case "global":
					next := cloneGlobal(before)
					next.ConnectionLimit = 42
					write <- service.SaveGlobal(ctx, "administrator", next)
				case "automatic lease":
					next := *before.AutomaticWhitelist
					next.DurationHours = 48
					write <- service.SaveAutomaticWhitelist(ctx, "administrator", next)
				case "database epoch":
					repo.version.Add(1)
					write <- nil
				default:
					clock.Store(at.Add(2 * time.Second).UnixNano())
					write <- nil
				}
			}()
			select {
			case err := <-write:
				if err != nil {
					t.Fatal("independent change failed", err)
				}
			case <-time.After(2 * time.Second):
				cancel()
				unblock()
				<-write
				t.Fatal("global validation held the writer lock")
			}
			unblock()
			select {
			case <-done:
				if !errors.Is(validationErr, ErrConflict) {
					t.Fatal("superseded validation committed", change, validationErr)
				}
			case <-ctx.Done():
				t.Fatal("superseded validation did not finish")
			}
			got, err := service.Global(context.Background())
			if err != nil || got.Telemetry.ReportSeconds != before.Telemetry.ReportSeconds {
				t.Fatal("uncommitted candidate was published", got.Telemetry, err)
			}
			if change == "global" && got.ConnectionLimit != 42 {
				t.Fatal("later global change was lost")
			}
			if change == "automatic lease" && got.AutomaticWhitelist.DurationHours != 48 {
				t.Fatal("later automatic lease change was lost")
			}
		})
	}
}
