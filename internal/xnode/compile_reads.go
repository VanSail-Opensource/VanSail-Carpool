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
	nodeID           string
	nodes            []Node
	nodesLoaded      bool
	endpoints        []Endpoint
	endpointsLoaded  bool
	groups           []Group
	groupsLoaded     bool
	identities       []Identity
	identitiesLoaded bool
	configurations   map[string]Configuration
	snippets         map[string]Snippet
	ddnsBindings     map[string]map[string]DDNSHostBinding
	nodeByID         map[string]Node
	users            map[string]model.User
	byID             map[int64]Identity
	byUser           map[string]Identity
	desired          map[string]*model.Purchase
	purchases        map[string][]model.Purchase
	periods          map[string]Period
	disabled         map[string][]string
	restricted       map[string]bool
}

func (s *Service) withCompileReads(node ...string) *Service {
	if _, ok := s.repo.(*compileReads); ok {
		return s
	}
	repo := &compileReads{Repository: s.repo, nodeByID: map[string]Node{}, users: map[string]model.User{}, byID: map[int64]Identity{}, byUser: map[string]Identity{}, desired: map[string]*model.Purchase{}, purchases: map[string][]model.Purchase{}, periods: map[string]Period{}, disabled: map[string][]string{}, restricted: map[string]bool{}}
	repo.configurations, repo.snippets = map[string]Configuration{}, map[string]Snippet{}
	repo.ddnsBindings = map[string]map[string]DDNSHostBinding{}
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
	next.topology = s.topology
	return next
}

func (r *compileReads) XNodeGroups(ctx context.Context) ([]Group, error) {
	if r.groupsLoaded {
		return r.groups, nil
	}
	items, err := r.Repository.XNodeGroups(ctx)
	if err == nil {
		r.groups, r.groupsLoaded = items, true
	}
	return items, err
}

func (r *compileReads) XNodeIdentities(ctx context.Context) ([]Identity, error) {
	if r.identitiesLoaded {
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
		if err = r.preloadMembers(ctx, items); err != nil {
			return nil, err
		}
		r.identities, r.identitiesLoaded = items, true
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
	if _, err := r.XNodeIdentities(ctx); err != nil {
		return Identity{}, err
	}
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
	if _, err := r.XNodeIdentities(ctx); err != nil {
		return Identity{}, err
	}
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
	if purchases, ok := r.purchases[id]; ok {
		for _, purchase := range purchases {
			if (purchase.Status == "active" || purchase.Status == "activating") && !purchase.ValidFrom.After(now) && purchase.ValidUntil.After(now) {
				r.desired[id] = &purchase
				return &purchase, nil
			}
		}
		r.desired[id] = nil
		return nil, nil
	}
	item, err := r.Repository.DesiredEntitlement(ctx, id, now)
	if err == nil {
		r.desired[id] = item
	}
	return item, err
}
