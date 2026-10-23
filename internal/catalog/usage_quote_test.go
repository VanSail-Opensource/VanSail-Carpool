package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type catalogUsageRemote struct {
	*catalogProxy
	report model.DashboardNodeUsage
	err    error
	userID string
	start  time.Time
	end    time.Time
}

func (r *catalogUsageRemote) NodeUsage(_ context.Context, userID string, start, end time.Time) (model.DashboardNodeUsage, error) {
	r.userID, r.start, r.end = userID, start, end
	return r.report, r.err
}

func TestNodeUsageValidatesRangeAndNormalizesDates(t *testing.T) {
	t.Parallel()
	remoteID := "remote-1"
	start := time.Date(2026, 8, 10, 23, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	end := start.Add(24 * time.Hour)
	remote := &catalogUsageRemote{catalogProxy: &catalogProxy{}, report: model.DashboardNodeUsage{Nodes: []model.DashboardNodeUsageNode{{UUID: "node-1"}}}}
	service := newCatalogServiceForTest(&catalogRepository{}, remote)
	report, err := service.NodeUsage(context.Background(), model.User{ProxyUserID: &remoteID}, start, end)
	if err != nil || report.StartDate != "2026-08-10" || report.EndDate != "2026-08-11" || remote.userID != remoteID || !remote.start.Equal(start.UTC()) {
		t.Fatalf("NodeUsage() = (%+v, %v), remote = %+v", report, err, remote)
	}

	testErr := errors.New("usage unavailable")
	tests := []struct {
		name   string
		user   model.User
		start  time.Time
		end    time.Time
		remote ProxyClient
		want   error
	}{
		{"zero start", model.User{ProxyUserID: &remoteID}, time.Time{}, end, remote, ErrDashboardNodeUsageRange},
		{"reversed", model.User{ProxyUserID: &remoteID}, end, start, remote, ErrDashboardNodeUsageRange},
		{"too wide", model.User{ProxyUserID: &remoteID}, start, start.Add(31 * 24 * time.Hour), remote, ErrDashboardNodeUsageRange},
		{"missing identity", model.User{}, start, end, remote, ErrDashboardNodeUsageUnavailable},
		{"missing provider", model.User{ProxyUserID: &remoteID}, start, end, &catalogProxy{}, ErrDashboardNodeUsageUnavailable},
		{"provider error", model.User{ProxyUserID: &remoteID}, start, end, &catalogUsageRemote{catalogProxy: &catalogProxy{}, err: testErr}, testErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newCatalogServiceForTest(&catalogRepository{}, test.remote).NodeUsage(context.Background(), test.user, test.start, test.end)
			if !errors.Is(err, test.want) {
				t.Fatalf("NodeUsage() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestQuoteRejectsInvalidSelectionsAndUnavailablePersistence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	user := model.User{ID: "user-1", OnboardingState: "complete"}
	remote := newCatalogNodeRemote()
	service := NewService(nodeCatalogRepository(), remote, time.Minute)
	for name, args := range map[string][]string{
		"missing combo": {"unknown"},
		"missing addon": {"core", "unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.Quote(ctx, user, args[0], args[1:], ""); !errors.Is(err, database.ErrNotFound) {
				t.Fatalf("Quote() error = %v, want ErrNotFound", err)
			}
		})
	}
	if _, err := service.Quote(ctx, model.User{ID: user.ID}, "core", nil, ""); err == nil {
		t.Fatal("Quote() accepted incomplete onboarding")
	}
	if _, err := NewService(&catalogRepository{}, &catalogProxy{}, time.Minute).Quote(ctx, user, "core", nil, ""); err == nil {
		t.Fatal("Quote() accepted a repository without quote support")
	}
	testErr := errors.New("quote failure")
	repository := &catalogQuoteRepository{catalogRepository: nodeCatalogRepository(), quoteErr: testErr}
	if _, err := NewService(repository, &catalogProxy{}, time.Minute).Quote(ctx, user, "core", nil, ""); !errors.Is(err, testErr) {
		t.Fatalf("Quote() error = %v, want %v", err, testErr)
	}
}

func TestPurchasesForwardsRepositoryResult(t *testing.T) {
	t.Parallel()
	testErr := errors.New("history failure")
	repository := &catalogRepository{purchases: []model.Purchase{{ID: "purchase-1"}}, purchasesErr: testErr}
	service := newCatalogServiceForTest(repository, &catalogProxy{})
	if _, err := service.Purchases(context.Background(), "user-1"); !errors.Is(err, testErr) {
		t.Fatalf("Purchases() error = %v, want %v", err, testErr)
	}
}
