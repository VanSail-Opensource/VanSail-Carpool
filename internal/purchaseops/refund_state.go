package purchaseops

import "context"

// RefundStateSource can provide purchase-scoped usage across traffic resets.
// Older providers retain their existing state contract.
type RefundStateSource interface {
	MemberRefundState(context.Context, string, string) (RemoteState, error)
}

func refundState(ctx context.Context, remote Remote, id, purchase string) (RemoteState, error) {
	if source, ok := remote.(RefundStateSource); ok {
		return source.MemberRefundState(ctx, id, purchase)
	}
	return remote.MemberOperationState(ctx, id)
}
