package purchaseops

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type refundQuiescenceRepository struct {
	WorkerRepository
	operation   providerops.Operation
	item        providerops.Item
	facts       PurchaseFacts
	completed   providerops.Completion
	finalized   int
	finalizeErr error
}

func newRefundQuiescenceRepository(status providerops.Status, now time.Time) *refundQuiescenceRepository {
	return &refundQuiescenceRepository{
		operation: providerops.Operation{Receipt: model.OperationReceipt{ID: "refund", Kind: OperationRefundKind, Status: "processing"}, OwnerUserID: "owner"},
		item:      providerops.Item{OperationID: "refund", Key: "purchase", TargetType: "purchase", TargetID: "purchase", Status: status},
		facts: PurchaseFacts{FirstTerm: true, Purchase: model.Purchase{
			ID: "purchase", UserID: "owner", Status: "active", PriceTXBMinor: 100,
			CreatedAt: now.Add(-time.Hour), ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(24 * time.Hour),
		}},
	}
}

func (r *refundQuiescenceRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{r.item}, nil
}

func (r *refundQuiescenceRepository) MemberPurchaseFacts(context.Context, string) (PurchaseFacts, error) {
	return r.facts, nil
}

func (r *refundQuiescenceRepository) UserByID(context.Context, string) (model.User, error) {
	remoteID := "remote"
	return model.User{ID: "owner", ProxyUserID: &remoteID}, nil
}

func (r *refundQuiescenceRepository) BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error) {
	r.item.Status = providerops.StatusProcessing
	return r.item, nil
}

func (r *refundQuiescenceRepository) CompleteProviderOperationItem(_ context.Context, _, _ string, completion providerops.Completion, _ time.Time) (providerops.Item, error) {
	r.item.Status, r.item.ErrorCode = completion.Status, completion.ErrorCode
	return r.item, nil
}

func (r *refundQuiescenceRepository) CompleteProviderOperation(_ context.Context, _ string, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.completed = completion
	r.operation.Receipt.Status = string(completion.Status)
	return r.operation, nil
}

func (r *refundQuiescenceRepository) FinalizeMemberRefund(context.Context, string, string, time.Time) (RefundResult, error) {
	r.finalized++
	return RefundResult{}, r.finalizeErr
}

type refundQuiescenceRemote struct {
	Remote
	states       []RemoteState
	stateReads   int
	quiesceCalls int
	restoreCalls int
	quiesceErr   error
	restoreErr   error
}

func (r *refundQuiescenceRemote) MemberOperationState(context.Context, string) (RemoteState, error) {
	index := r.stateReads
	r.stateReads++
	if index >= len(r.states) {
		index = len(r.states) - 1
	}
	return r.states[index], nil
}

func (r *refundQuiescenceRemote) QuiesceMemberOperation(context.Context, string) error {
	r.quiesceCalls++
	return r.quiesceErr
}

func (r *refundQuiescenceRemote) RestoreMemberOperation(context.Context, string, model.Purchase) error {
	r.restoreCalls++
	return r.restoreErr
}

func (r *refundQuiescenceRemote) DefinitiveMutationFailure(error) bool {
	return false
}
