package connections

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPollConnectionScanRejectsAnotherProfileTarget(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository := &targetScopedScanRepository{scan: providerops.ConnectionScan{ID: "scan", UserID: "target", Status: providerops.StatusProcessing, ExpiresAt: now.Add(time.Minute)}}
	service := NewService(repository, nil, nil)
	service.now = func() time.Time { return now }
	if _, err := service.Poll(context.Background(), "other", "scan"); !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("Poll(other profile) = %v, want ErrScanNotFound", err)
	}
	if repository.lastOwnerID != "other" {
		t.Fatalf("poll owner scope = %q, want other", repository.lastOwnerID)
	}
}

type targetScopedScanRepository struct {
	scan        providerops.ConnectionScan
	lastOwnerID string
}

func (*targetScopedScanRepository) CreateConnectionScan(context.Context, providerops.ConnectionScanInput, time.Time) (providerops.ConnectionScan, bool, error) {
	return providerops.ConnectionScan{}, false, errors.New("unexpected")
}
func (r *targetScopedScanRepository) ConnectionScanForUser(_ context.Context, scanID, userID string) (providerops.ConnectionScan, error) {
	r.lastOwnerID = userID
	if scanID != r.scan.ID || userID != r.scan.UserID {
		return providerops.ConnectionScan{}, errors.New("not found")
	}
	return r.scan, nil
}
func (*targetScopedScanRepository) ConnectionScanByID(context.Context, string) (providerops.ConnectionScan, error) {
	return providerops.ConnectionScan{}, errors.New("unexpected")
}
func (*targetScopedScanRepository) UpdateConnectionScan(context.Context, string, providerops.ConnectionScanUpdate, time.Time) (providerops.ConnectionScan, error) {
	return providerops.ConnectionScan{}, errors.New("unexpected")
}
func (*targetScopedScanRepository) UserByID(context.Context, string) (model.User, error) {
	return model.User{}, errors.New("unexpected")
}
