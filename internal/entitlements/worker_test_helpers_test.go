package entitlements

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

type entitlementRepository struct {
	jobs               []*model.OutboxJob
	claimIndex         int
	claimErr           error
	completeErr        error
	purchase           model.Purchase
	purchaseErr        error
	user               model.User
	userForPurchaseErr error
	userByIDErr        error
	desired            *model.Purchase
	desiredErr         error
	continuity         *model.Purchase
	continuityErr      error
	resetPhase         string
	resetPhaseErr      error
	advanceResetErr    error
	resetTransitions   [][2]string
	markErr            error
	markedPurchaseID   string
	markedSuccess      *bool
	expiredPurchaseID  string
	expireErr          error
	completedJobID     string
	completedAttempts  int
	completedErr       error
	releaseCalls       int
	releaseErr         error
}

func (r *entitlementRepository) ClaimOutboxJob(context.Context, time.Time) (*model.OutboxJob, error) {
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	if r.claimIndex >= len(r.jobs) {
		return nil, nil
	}
	job := r.jobs[r.claimIndex]
	r.claimIndex++
	return job, nil
}
func (r *entitlementRepository) CompleteOutboxJob(_ context.Context, jobID string, attempts int, jobErr error, _ time.Time) error {
	r.completedJobID, r.completedAttempts, r.completedErr = jobID, attempts, jobErr
	return r.completeErr
}
func (r *entitlementRepository) PurchaseByID(context.Context, string) (model.Purchase, error) {
	return r.purchase, r.purchaseErr
}
func (r *entitlementRepository) UserForPurchase(context.Context, string) (model.User, error) {
	return r.user, r.userForPurchaseErr
}
func (r *entitlementRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, r.userByIDErr
}
func (r *entitlementRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return r.desired, r.desiredErr
}
func (r *entitlementRepository) ContinuityEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return r.continuity, r.continuityErr
}
func (r *entitlementRepository) PurchaseTrafficResetPhase(context.Context, string) (string, error) {
	if r.resetPhaseErr != nil {
		return "", r.resetPhaseErr
	}
	if r.resetPhase == "" {
		return "pending", nil
	}
	return r.resetPhase, nil
}
func (r *entitlementRepository) AdvancePurchaseTrafficReset(_ context.Context, _ string, from, to string, _ time.Time) error {
	if r.advanceResetErr != nil {
		return r.advanceResetErr
	}
	r.resetTransitions = append(r.resetTransitions, [2]string{from, to})
	r.resetPhase = to
	return nil
}
func (r *entitlementRepository) MarkPurchaseSyncResult(_ context.Context, purchaseID string, success bool, _ time.Time) error {
	r.markedPurchaseID = purchaseID
	r.markedSuccess = new(bool)
	*r.markedSuccess = success
	return r.markErr
}
func (r *entitlementRepository) ExpirePurchase(_ context.Context, purchaseID string, _ time.Time) error {
	r.expiredPurchaseID = purchaseID
	return r.expireErr
}
func (r *entitlementRepository) ReleaseUserSyncNotifications(context.Context, string, time.Time) error {
	r.releaseCalls++
	return r.releaseErr
}

type entitlementRemnawave struct {
	applyCalls         int
	applyUserID        string
	applyLimit         int64
	applyResetStrategy string
	applySquads        []string
	applyExpiresAt     time.Time
	applyErr           error
	removeCalls        int
	removeErr          error
	resetCalls         int
	resetErr           error
}

func (r *entitlementRemnawave) ApplyEntitlement(_ context.Context, userID string, limit int64, strategy string, squads []string, expiresAt time.Time) error {
	r.applyCalls++
	r.applyUserID, r.applyLimit, r.applyResetStrategy = userID, limit, strategy
	r.applySquads = append([]string(nil), squads...)
	r.applyExpiresAt = expiresAt
	return r.applyErr
}
func (r *entitlementRemnawave) ResetTraffic(context.Context, string) error {
	r.resetCalls++
	return r.resetErr
}
func (r *entitlementRemnawave) RemoveEntitlement(context.Context, string) error {
	r.removeCalls++
	return r.removeErr
}

func newEntitlementWorkerForTest(repository Repository, remnawave RemnawaveClient) *Worker {
	worker := NewWorker(repository, remnawave)
	worker.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
	return worker
}
