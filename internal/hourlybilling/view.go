package hourlybilling

import (
	"context"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// View projects safe current metadata and owner-scoped committed accounting.
// It neither creates a daily allowance nor grants access or paid-extra consent.
func (s *Service) View(ctx context.Context, user model.User) (model.HourlyAccountView, error) {
	owner, err := s.currentHourlyOwner(ctx, user)
	if err != nil {
		return model.HourlyAccountView{}, err
	}
	if s.policy == nil {
		return model.HourlyAccountView{}, ErrUnavailable
	}
	view := model.HourlyAccountView{Squads: []model.HourlySquadView{}}
	if view.Policy, err = s.policy.HourlyPolicy(ctx); err != nil {
		return view, err
	}
	if err = model.ValidateHourlyPolicy(view.Policy.Policy); err != nil {
		return view, err
	}
	if view.Balance, err = s.store.Balance(ctx, owner.ID); err != nil {
		return view, err
	}
	now := s.now().UTC()
	if view.LegacyBlocked, err = s.store.HourlyLegacyBlocked(ctx, owner.ID, now); err != nil {
		return view, err
	}
	if view.ExtraPreference, err = s.store.HourlyExtraPreference(ctx, owner.ID, owner.AccountGeneration); err != nil {
		return view, err
	}
	if view.Usage, err = s.hourlyAccountUsage(ctx, owner, view.Policy.Policy, now); err != nil {
		return view, err
	}
	capacity, err := s.store.HourlyOwnerCapacityAvailable(ctx, owner.ID, owner.AccountGeneration)
	if err != nil {
		return view, err
	}
	offers, err := s.store.ListHourlyOffers(ctx)
	if err != nil {
		return view, err
	}
	enrollments, err := s.store.HourlyViewEnrollments(ctx, owner.ID, owner.AccountGeneration, now)
	if err != nil {
		return view, err
	}
	products, err := s.store.ListSquadProductOverrides(ctx, false)
	if err != nil {
		return view, err
	}
	stock, err := s.store.HourlyStockRemaining(ctx)
	if err != nil {
		return view, err
	}
	topology := xnode.CatalogTopology{}
	if s.native != nil {
		topology, err = s.native.CatalogTopology(ctx)
	} else {
		topology.Groups, err = s.store.XNodeGroups(ctx)
	}
	if err != nil {
		return view, err
	}
	latest, cooldowns := latestHourlyView(enrollments)
	byOffer, byProduct, byGroup := map[string]model.HourlyOffer{}, map[string]model.SquadProduct{}, map[string]xnode.Group{}
	for _, offer := range offers {
		byOffer[offer.GroupID] = offer
	}
	for group := range latest {
		if _, exists := byOffer[group]; !exists {
			byOffer[group] = model.HourlyOffer{GroupID: group, HourlyPrice: model.TXBMoney(0)}
		}
	}
	for _, product := range products {
		byProduct[product.GroupID] = product
	}
	for _, group := range topology.Groups {
		byGroup[group.ID] = group
	}
	for group, offer := range byOffer {
		if !offer.Enabled && latest[group] == nil {
			continue
		}
		row := model.HourlySquadView{Offer: offer, Enrollment: latest[group], CooldownUntil: cooldowns[group]}
		row.Squad = hourlySquadMetadata(offer, byProduct[group], byGroup[group], topology.Nodes)
		if remaining, limited := stock[group]; limited {
			row.Squad.StockRemaining = &remaining
		}
		row.Squad.StockHeldByCurrentUser = row.Enrollment != nil && row.Enrollment.Status == model.HourlyActive
		minimum, minimumErr := model.HourlyActivationMinimum(view.Policy.Policy, offer.RequiredHours, offer.HourlyPriceTXBMinor)
		if minimumErr == nil {
			money := model.TXBMoney(minimum)
			row.RequiredBalance = &money
		}
		row.ReasonCode = s.hourlyViewReason(ctx, owner, view, row, minimum, minimumErr, capacity, now)
		row.Ready = row.ReasonCode == ""
		view.Squads = append(view.Squads, row)
		if err := ctx.Err(); err != nil {
			return model.HourlyAccountView{}, err
		}
	}
	sort.Slice(view.Squads, func(i, j int) bool { return view.Squads[i].Offer.GroupID < view.Squads[j].Offer.GroupID })
	_, err = s.currentHourlyOwner(ctx, owner)
	return view, err
}

func latestHourlyView(enrollments []model.HourlyEnrollment) (map[string]*model.HourlyEnrollment, map[string]*time.Time) {
	latest, cooldowns := map[string]*model.HourlyEnrollment{}, map[string]*time.Time{}
	for _, value := range enrollments {
		e := value
		if latest[e.GroupID] == nil || e.CreatedAt.After(latest[e.GroupID].CreatedAt) || e.CreatedAt.Equal(latest[e.GroupID].CreatedAt) && e.ID > latest[e.GroupID].ID {
			latest[e.GroupID] = &e
		}
		if e.CooldownUntil != nil && (cooldowns[e.GroupID] == nil || e.CooldownUntil.After(*cooldowns[e.GroupID])) {
			cooldowns[e.GroupID] = e.CooldownUntil
		}
	}
	return latest, cooldowns
}

func hourlySquadMetadata(offer model.HourlyOffer, stored model.SquadProduct, group xnode.Group, nodes []xnode.TopologyNode) model.SquadProduct {
	p := model.SquadProduct{ID: offer.GroupID, GroupID: offer.GroupID, Name: group.Name, Description: stored.Description,
		Features: stored.Features, Profile: stored.Profile, Price: offer.HourlyPrice, Visible: offer.Enabled,
		StockLimit: stored.StockLimit, StockRemaining: stored.StockRemaining, ActivationRequired: stored.ActivationRequired,
		UpstreamPresent: group.ID != "" && group.Enabled, AccessibleNodes: []model.CatalogNode{}, CreatedAt: offer.CreatedAt, UpdatedAt: offer.UpdatedAt}
	if p.Name == "" {
		p.Name = offer.GroupID
	}
	selected := map[string]bool{}
	for _, binding := range group.Bindings {
		selected[binding.NodeID] = true
	}
	for _, node := range nodes {
		if selected[node.ID] && node.Enabled {
			p.AccessibleNodes = append(p.AccessibleNodes, model.CatalogNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, ConsumptionMultiplier: 1})
		}
	}
	return p
}

func (s *Service) hourlyViewReason(ctx context.Context, owner model.User, view model.HourlyAccountView, row model.HourlySquadView, minimum int64, minimumErr error, capacity bool, now time.Time) string {
	if !row.Squad.UpstreamPresent || !row.Offer.Enabled || minimumErr != nil {
		return "unavailable"
	}
	if row.CooldownUntil != nil && row.CooldownUntil.After(now) {
		return "cooldown"
	}
	if view.LegacyBlocked {
		return "legacy_blocked"
	}
	if !view.Policy.Policy.SalesEnabled {
		return "sales_paused"
	}
	if !capacity {
		return "global_capacity"
	}
	held := row.Enrollment != nil && row.Enrollment.Status == model.HourlyActive
	if row.Squad.StockRemaining != nil && *row.Squad.StockRemaining == 0 && !held {
		return "capacity"
	}
	if view.Balance.MinorInt64() < minimum {
		return "insufficient_balance"
	}
	if s.native == nil || !s.store.HourlyTrafficHistoryReady() {
		return "agent_incompatible"
	}
	ready, err := s.native.HourlySquadReadiness(ctx, row.Offer.GroupID, owner.ID)
	if err == nil && ready.Code == "XNODE_HOURLY_CAPACITY" {
		return "global_capacity"
	}
	if err != nil || !ready.Ready {
		return "agent_incompatible"
	}
	return ""
}
