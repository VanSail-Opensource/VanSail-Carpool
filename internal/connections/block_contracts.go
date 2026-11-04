package connections

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"time"
)

// BlockWorkerRepository owns active block and operation lifecycle state.
type BlockWorkerRepository interface {
	UserByID(context.Context, string) (model.User, error)
	ConnectionIPBlockByOperation(context.Context, string) (IPBlockRecord, error)
	ProviderOperationItems(context.Context, string) ([]providerops.Item, error)
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error)
	CompleteConnectionIPBlockOperation(context.Context, string, string, string, BlockOperationCompletion, time.Time) error
}

// BlockOperationCompletion atomically closes an operation and transitions its active block row.
type BlockOperationCompletion struct {
	Operation    providerops.Completion
	ItemStatus   providerops.Status
	BlockStatus  string
	RemoveBlock  bool
	ClearUnblock bool
}

// BlockRemote performs plugin execution and disconnect reconciliation via the queue.
type BlockRemote interface {
	UnblockIP(context.Context, string, string) error
	DefinitiveMutationFailure(error) bool
}
