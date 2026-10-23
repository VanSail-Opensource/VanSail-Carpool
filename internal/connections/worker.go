package connections

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// Worker starts connection scans without retrying an ambiguous provider POST.
type Worker struct {
	repository Repository
	remote     Remote
	now        func() time.Time
}

// NewWorker creates a connection scan outbox handler.
func NewWorker(repository Repository, remote Remote) *Worker {
	return &Worker{repository: repository, remote: remote, now: time.Now}
}

// HandleOutbox starts a queued scan or resolves an interrupted start safely.
func (w *Worker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	scanID, err := jobpayload.TargetID(job, "scanId")
	if err != nil {
		return err
	}
	record, err := w.repository.ConnectionScanByID(ctx, scanID)
	if err != nil {
		return fmt.Errorf("load connection scan: %w", err)
	}
	if providerops.Terminal(record.Status) {
		return nil
	}
	now := w.now().UTC()
	if record.Status == providerops.StatusProcessing {
		if record.ProviderJobID != "" {
			return nil
		}
		return w.pendingReview(ctx, record, now)
	}
	record, err = w.repository.UpdateConnectionScan(ctx, record.ID, providerops.ConnectionScanUpdate{Status: providerops.StatusProcessing}, now)
	if err != nil {
		return err
	}
	user, err := w.repository.UserByID(ctx, record.UserID)
	if err != nil {
		return w.fail(ctx, record.ID, "CONNECTION_SCAN_FAILED", now, err)
	}
	var providerJobID string
	var requestErr error
	if record.RequestFingerprint == nativeScanFingerprint() {
		native, ok := w.remote.(NativeRemote)
		if !ok {
			return w.fail(ctx, record.ID, "XNODE_UNAVAILABLE", now, nil)
		}
		providerJobID, requestErr = native.RequestNativeConnectionScan(ctx, user.ID)
		if requestErr != nil {
			return w.fail(ctx, record.ID, "XNODE_UNAVAILABLE", now, requestErr)
		}
	} else if user.ProxyUserID == nil {
		return w.fail(ctx, record.ID, "XNODE_IDENTITY_REQUIRED", now, err)
	} else {
		providerJobID, requestErr = w.remote.RequestConnectionScan(ctx, *user.ProxyUserID)
	}
	if requestErr != nil {
		return w.pendingReview(ctx, record, now)
	}
	_, err = w.repository.UpdateConnectionScan(ctx, record.ID, providerops.ConnectionScanUpdate{
		Status: providerops.StatusProcessing, ProviderJobID: providerJobID,
	}, now)
	return err
}

func (w *Worker) pendingReview(ctx context.Context, scan providerops.ConnectionScan, now time.Time) error {
	_, err := w.repository.UpdateConnectionScan(ctx, scan.ID, providerops.ConnectionScanUpdate{
		Status: providerops.StatusPendingReview, ProgressPercent: scan.ProgressPercent,
		ErrorCode: "CONNECTION_SCAN_START_AMBIGUOUS",
	}, now)
	return err
}

func (w *Worker) fail(ctx context.Context, scanID, code string, now time.Time, cause error) error {
	_, updateErr := w.repository.UpdateConnectionScan(ctx, scanID, providerops.ConnectionScanUpdate{Status: providerops.StatusFailed, ErrorCode: code}, now)
	if updateErr != nil {
		return updateErr
	}
	return cause
}
