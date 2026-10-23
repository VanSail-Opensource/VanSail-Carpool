package connections

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// DropWorkerRepository is the durable selected-IP operation surface.
type DropWorkerRepository interface {
	UserByID(context.Context, string) (model.User, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
}

// DropRemote performs unlink and reconciliation scans through the provider queue.
type DropRemote interface {
	RequestConnectionScan(context.Context, string) (string, error)
	PollConnectionScan(context.Context, string) (ProviderScan, error)
	DropConnection(context.Context, string, string) error
	DefinitiveMutationFailure(error) bool
}

// DropWorker processes one encrypted selected-IP target without blind retries.
type DropWorker struct {
	repository DropWorkerRepository
	remote     DropRemote
	signer     *Signer
	secrets    SecretBox
	now        func() time.Time
}

// NewDropWorker creates a connection drop operation handler.
func NewDropWorker(repository DropWorkerRepository, remote DropRemote, signer *Signer, secrets SecretBox) *DropWorker {
	return &DropWorker{repository: repository, remote: remote, signer: signer, secrets: secrets, now: time.Now}
}

// HandleProviderOperation drops once or reconciles an already-started target.
func (w *DropWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	operation, item, claims, err := w.prepare(ctx, operation, job)
	if err != nil {
		return err
	}
	if item.Status != providerops.StatusQueued && item.Status != providerops.StatusProcessing {
		return w.finish(ctx, operation, item, item.Status, item.ErrorCode)
	}
	user, err := w.repository.UserByID(ctx, operation.OwnerUserID)
	if err != nil || user.ProxyUserID == nil {
		return w.finish(ctx, operation, item, providerops.StatusFailed, "XNODE_IDENTITY_REQUIRED")
	}
	if item.Status == providerops.StatusProcessing {
		return w.reconcile(ctx, operation, item, claims, *user.ProxyUserID, nil)
	}
	item, err = w.repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, item.Key, w.now().UTC())
	if err != nil {
		return err
	}
	scoped, callErr := scopedControl(ctx, w.remote, operation.OwnerUserID, claims.IP, claims.NodeUUID, true, 0, false)
	if !scoped {
		callErr = w.remote.DropConnection(ctx, claims.IP, claims.NodeUUID)
	}
	if callErr == nil {
		return w.finish(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	return w.reconcile(ctx, operation, item, claims, *user.ProxyUserID, callErr)
}

func (w *DropWorker) prepare(ctx context.Context, operation providerops.Operation, job model.OutboxJob) (providerops.Operation, providerops.Item, HandleClaims, error) {
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return providerops.Operation{}, providerops.Item{}, HandleClaims{}, err
		}
	}
	items, err := w.repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].TargetType != "connection_handle_sha256" {
		return providerops.Operation{}, providerops.Item{}, HandleClaims{}, errors.New("connection drop has an invalid target")
	}
	sealed, err := jobpayload.TargetID(job, "sealedTarget")
	if err != nil {
		return providerops.Operation{}, providerops.Item{}, HandleClaims{}, err
	}
	plaintext, err := w.secrets.Open(dropSecretContext(operation.OwnerUserID, operation.RequestFingerprint), sealed)
	if err != nil || hash(string(plaintext)) != items[0].TargetID {
		return providerops.Operation{}, providerops.Item{}, HandleClaims{}, errors.New("connection drop target could not be verified")
	}
	claims, err := w.signer.Verify(string(plaintext), operation.OwnerUserID, operation.Receipt.CreatedAt)
	return operation, items[0], claims, err
}

func (w *DropWorker) reconcile(ctx context.Context, operation providerops.Operation, item providerops.Item, claims HandleClaims, remoteID string, callErr error) error {
	jobID, err := w.remote.RequestConnectionScan(ctx, remoteID)
	if err != nil {
		return w.finish(ctx, operation, item, providerops.StatusPendingReview, "DROP_RECONCILIATION_FAILED")
	}
	result, completed := w.pollReconciliation(ctx, jobID)
	if !completed {
		return w.finish(ctx, operation, item, providerops.StatusPendingReview, "DROP_OUTCOME_AMBIGUOUS")
	}
	if !connectionPresent(result, claims.NodeUUID, claims.IP) {
		return w.finish(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	if callErr != nil && w.remote.DefinitiveMutationFailure(callErr) {
		return w.finish(ctx, operation, item, providerops.StatusFailed, "DROP_REJECTED")
	}
	return w.finish(ctx, operation, item, providerops.StatusPendingReview, "DROP_OUTCOME_AMBIGUOUS")
}
