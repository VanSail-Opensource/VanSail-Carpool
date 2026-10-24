package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

const PAYGJob = "xnode_payg_reconcile"
const PAYGRecoveryJob = "xnode_payg_recover"
const PAYGGB int64 = 1 << 30
const PAYGStartBytes int64 = 10 << 10
const paygPrefix = "member-payg-"

type PAYGAccount struct {
	ProxyUserID                                             int64
	UserID, GroupID, NodeID, PeriodID                       string
	PurchasedBytes, UsedBytes, UnitPriceMinor, BalanceMinor int64
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
	XNodeIdentityByService(context.Context, string) (Identity, error)
	XNodePAYGAccounts(context.Context, string, string) ([]PAYGAccount, error)
	XNodePAYGAccount(context.Context, int64) (PAYGAccount, error)
	SaveXNodePAYGAccount(context.Context, PAYGAccount) error
	ObserveXNodePAYGConnections(context.Context, string, []int64, time.Time) error
	XNodePAYGUsage(context.Context, string, time.Time, time.Time) ([]PAYGUsage, error)
	ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error)
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
