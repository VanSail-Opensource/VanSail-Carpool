package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

// compileReads batches shared inputs for one compilation. It is never reused
// across transactions or goroutines; the epoch check rejects concurrent changes.
type compileReads struct {
	Repository
	nodeID     string
	groups     []Group
	identities []Identity
	users      map[string]model.User
	byID       map[int64]Identity
	byUser     map[string]Identity
	desired    map[string]*model.Purchase
	purchases  map[string][]model.Purchase
	periods    map[string]Period
}

func (s *Service) withCompileReads(node ...string) *Service {
	if _, ok := s.repo.(*compileReads); ok {
		return s
	}
	repo := &compileReads{Repository: s.repo, users: map[string]model.User{}, byID: map[int64]Identity{}, byUser: map[string]Identity{}, desired: map[string]*model.Purchase{}, purchases: map[string][]model.Purchase{}, periods: map[string]Period{}}
	if len(node) > 0 {
		repo.nodeID = node[0]
	}
	next := NewService(repo, s.settings, s.vault, s.publicURL, s.validationQueue)
	next.master, next.now = s.master, s.now
	next.policyAssets = s.policyAssets
	next.routingAssets = s.routingAssets
	next.routingOverrides = s.routingOverrides
	next.routingRuntime = s.routingRuntime
	next.compilation = s.compilation
	return next
}

func (r *compileReads) XNodeGroups(ctx context.Context) ([]Group, error) {
	if r.groups != nil {
		return r.groups, nil
	}
	items, err := r.Repository.XNodeGroups(ctx)
	if err == nil {
		r.groups = items
	}
	return items, err
}
func (r *compileReads) XNodeIdentities(ctx context.Context) ([]Identity, error) {
	if r.identities != nil {
		return r.identities, nil
	}
	var items []Identity
	var err error
	if source, ok := r.Repository.(interface {
		XNodeIdentitiesForNode(context.Context, string) ([]Identity, error)
	}); ok && r.nodeID != "" {
		items, err = source.XNodeIdentitiesForNode(ctx, r.nodeID)
	} else {
		items, err = r.Repository.XNodeIdentities(ctx)
	}
	if err == nil {
		r.identities = items
		for _, item := range items {
			r.byID[item.ProxyUserID] = item
			if item.UserID != "" {
				r.byUser[item.UserID] = item
			}
		}
	}
	return items, err
}
func (r *compileReads) XNodeIdentity(ctx context.Context, id int64) (Identity, error) {
	items, err := r.XNodeIdentities(ctx)
	if err != nil {
		return Identity{}, err
	}
	_ = items
	if item, ok := r.byID[id]; ok {
		return item, nil
	}
	item, err := r.Repository.XNodeIdentity(ctx, id)
	if err == nil {
		r.byID[id] = item
	}
	return item, err
}
func (r *compileReads) XNodeIdentityByUser(ctx context.Context, id string) (Identity, error) {
	items, err := r.XNodeIdentities(ctx)
	if err != nil {
		return Identity{}, err
	}
	_ = items
	if item, ok := r.byUser[id]; ok {
		return item, nil
	}
	return Identity{}, ErrNotFound
}
func (r *compileReads) UserByID(ctx context.Context, id string) (model.User, error) {
	if item, ok := r.users[id]; ok {
		return item, nil
	}
	item, err := r.Repository.UserByID(ctx, id)
	if err == nil {
		r.users[id] = item
	}
	return item, err
}
func (r *compileReads) XNodePeriod(ctx context.Context, id string) (Period, error) {
	if item, ok := r.periods[id]; ok {
		return item, nil
	}
	item, err := r.Repository.XNodePeriod(ctx, id)
	if err == nil {
		r.periods[id] = item
	}
	return item, err
}

func (r *compileReads) ListPurchases(ctx context.Context, id string) ([]model.Purchase, error) {
	if item, ok := r.purchases[id]; ok {
		return item, nil
	}
	item, err := r.Repository.ListPurchases(ctx, id)
	if err == nil {
		r.purchases[id] = item
	}
	return item, err
}
func (r *compileReads) DesiredEntitlement(ctx context.Context, id string, now time.Time) (*model.Purchase, error) {
	if item, ok := r.desired[id]; ok {
		return item, nil
	}
	item, err := r.Repository.DesiredEntitlement(ctx, id, now)
	if err == nil {
		r.desired[id] = item
	}
	return item, err
}
