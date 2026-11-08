// Package hourlybilling coordinates commerce with the existing native and job queues.
package hourlybilling

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const (
	ActivateOperation = "xnode_hourly_activate"
	DisableOperation  = "xnode_hourly_disable"
	SettlementJob     = "xnode_hourly_settle"
)

var ErrUnavailable = errors.New("hourly commerce is unavailable")

type PolicySource interface {
	HourlyPolicy(context.Context) (model.HourlyPolicySnapshot, error)
}

type nativeCoordinator interface {
	CatalogTopology(context.Context) (xnode.CatalogTopology, error)
	HourlySquadReadiness(context.Context, string, ...string) (xnode.HourlyReadiness, error)
	PrepareHourlyPrincipal(context.Context, string) (xnode.HourlyPrincipal, error)
	PreflightHourlyEnrollment(context.Context, string) (xnode.HourlyReadiness, error)
	SyncHourlyEnrollment(context.Context, string) error
	ResumeHourlyOwner(context.Context, string, int64) error
	CloseHourlyOwnerIfIdle(context.Context, string, int64, string) error
}

type fundScope struct {
	owner      string
	generation int64
	day        string
}

type fundIntent struct {
	id       string
	snapshot model.HourlyPolicySnapshot
	input    database.HourlyExtraFundingInput
	running  bool
}

// Service contains no competing queue. Commands use providerops; renewals use
// the shared outbox; native delivery uses the existing per-node executor.
type Service struct {
	store            *database.Store
	policy           PolicySource
	native           nativeCoordinator
	vault            *secret.Vault
	now              func() time.Time
	mu               sync.Mutex
	funds            map[fundScope]fundIntent
	dueAfter         string
	deliveryCursors  map[string]deliveryCursor
	deliverySequence uint64
}

func New(store *database.Store, policy PolicySource, native *xnode.Service, vault *secret.Vault) *Service {
	s := &Service{store: store, policy: policy, vault: vault, now: time.Now, funds: map[fundScope]fundIntent{}, deliveryCursors: map[string]deliveryCursor{}}
	if native != nil {
		s.native = native
	}
	return s
}
