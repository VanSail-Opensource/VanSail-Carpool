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
	groups     []Group
	identities []Identity
	users      map[string]model.User
	byID       map[int64]Identity
	byUser     map[string]Identity
	desired    map[string]*model.Purchase
	purchases  map[string][]model.Purchase
	periods    map[string]Period
	migration  *MigrationState
}

func (s *Service) withCompileReads() *Service {
	if _, ok := s.repo.(*compileReads); ok {
		return s
	}
	repo := &compileReads{Repository: s.repo, users: map[string]model.User{}, byID: map[int64]Identity{}, byUser: map[string]Identity{}, desired: map[string]*model.Purchase{}, purchases: map[string][]model.Purchase{}, periods: map[string]Period{}}
	next := NewService(repo, s.settings, s.vault, s.publicURL, s.validationQueue)
	next.master, next.now, next.legacyProxies = s.master, s.now, s.legacyProxies
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
	items, err := r.Repository.XNodeIdentities(ctx)
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
	return Identity{}, ErrNotFound
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
func (r *compileReads) XNodeMigration(ctx context.Context) (MigrationState, error) {
	if r.migration != nil {
		return *r.migration, nil
	}
	item, err := r.Repository.XNodeMigration(ctx)
	if err == nil {
		r.migration = &item
	}
	return item, err
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
