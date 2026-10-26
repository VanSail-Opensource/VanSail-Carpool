package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
)

const PAYGJob = "xnode_payg_reconcile"
const PAYGRecoveryJob = "xnode_payg_recover"
const PAYGGB int64 = 1 << 30
const PAYGStartBytes int64 = 100 << 20
const PAYGWindow = 72 * time.Hour
const paygPrefix = "member-payg-"

type PAYGAccount struct {
	PersonalPricing                                         bool `json:"-"`
	Disabled                                                bool
	ProxyUserID                                             int64
	UserID, GroupID, NodeID, PeriodID                       string
	PurchasedBytes, UsedBytes, UnitPriceMinor, BalanceMinor int64
	CreditBytes                                             int64
	CreditStartedAt                                         time.Time
}

// HasCredit excludes unused GB credit at its exact 72-hour expiry.
func (a PAYGAccount) HasCredit(at time.Time) bool {
	return a.CreditBytes > 0 && !a.CreditStartedAt.IsZero() && at.Before(a.CreditStartedAt.Add(PAYGWindow))
}

// Funded is shared by member status and volatile node admission.
func (a PAYGAccount) Funded(at time.Time) bool {
	return a.BalanceMinor >= 0 && (a.HasCredit(at) || a.BalanceMinor >= a.UnitPriceMinor)
}

type PAYGUsage struct {
	GroupID                  string
	TrafficBytes, SpentMinor int64
}
type PAYGSquad struct {
	Squad        model.SquadProduct `json:"squad"`
	Status       string             `json:"status"`
	TrafficBytes string             `json:"trafficBytes"`
	Spent        model.Money        `json:"spent"`
}
type PAYGView struct {
	Balance      model.Money `json:"balance"`
	WindowStart  time.Time   `json:"windowStart"`
	AsOf         time.Time   `json:"asOf"`
	Squads       []PAYGSquad `json:"squads"`
	TrafficBytes string      `json:"trafficBytes"`
	Spent        model.Money `json:"spent"`
}

type paygRepository interface {
	XNodePAYGPreferences(context.Context, string) (preferences.PAYG, error)
	XNodeIdentityByService(context.Context, string) (Identity, error)
	XNodePAYGAccounts(context.Context, string, string) ([]PAYGAccount, error)
	XNodePAYGAccount(context.Context, int64) (PAYGAccount, error)
	SaveXNodePAYGAccount(context.Context, PAYGAccount) error
	ObserveXNodePAYGConnections(context.Context, string, []int64, time.Time) error
	XNodePAYGUsage(context.Context, string, time.Time, time.Time) ([]PAYGUsage, error)
	ListSquadProductOverrides(context.Context, bool) ([]model.SquadProduct, error)
	Balance(context.Context, string) (model.Money, error)
	EnqueueOutbox(context.Context, string, string, time.Time) error
}

func (s *Service) paygRepository() (paygRepository, bool) {
	repo := s.repo
	if reads, ok := repo.(*compileReads); ok {
		repo = reads.Repository
	}
	r, ok := repo.(paygRepository)
	return r, ok
}
