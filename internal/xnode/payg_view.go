package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"math"
	"math/big"
	"strconv"
	"time"
)

func (s *Service) OwnedPAYG(ctx context.Context, user string) (PAYGView, error) {
	asOf := s.now().UTC().Truncate(time.Minute)
	out := PAYGView{AsOf: asOf, WindowStart: asOf.Add(-72 * time.Hour), Squads: []PAYGSquad{}, TrafficBytes: "0", Spent: model.TXBMoney(0)}
	r, ok := s.paygRepository()
	if !ok {
		return out, ErrUnavailable
	}
	member, err := s.repo.UserByID(ctx, user)
	if err != nil {
		return out, err
	}
	if member.OnboardingState != "complete" {
		return out, ErrInvalid
	}
	out.Balance, err = r.Balance(ctx, user)
	if err != nil {
		return out, err
	}
	products, err := r.ListSquadProducts(ctx, false)
	if err != nil {
		return out, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return out, err
	}
	groupByID := map[string]Group{}
	for _, g := range groups {
		groupByID[g.ID] = g
	}
	usage, err := r.XNodePAYGUsage(ctx, user, out.WindowStart, asOf)
	if err != nil {
		return out, err
	}
	byGroup := map[string]PAYGUsage{}
	for _, u := range usage {
		byGroup[u.GroupID] = u
	}
	accounts, err := r.XNodePAYGAccounts(ctx, user, "")
	if err != nil {
		return out, err
	}
	seenProducts := map[string]bool{}
	for _, p := range products {
		seenProducts[p.GroupID] = true
	}
	for _, u := range usage {
		if seenProducts[u.GroupID] {
			continue
		}
		products = append(products, model.SquadProduct{ID: u.GroupID, GroupID: u.GroupID, BillingMode: "combo", Price: model.TXBMoney(0)})
	}
	coverage, err := s.paygCoverage(ctx, user)
	if err != nil {
		return out, err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, user)
	if err != nil {
		return out, err
	}
	if parent, e := s.repo.XNodeIdentityByUser(ctx, user); e == nil {
		restricted = restricted || parent.AccessDisabled
	} else if !isMissing(e) {
		return out, e
	}
	total := new(big.Int)
	spent := int64(0)
	for _, p := range products {
		u := byGroup[p.GroupID]
		if p.BillingMode != "payg" && u.TrafficBytes == 0 && u.SpentMinor == 0 {
			continue
		}
		g := groupByID[p.GroupID]
		p.Name = g.Name
		p.AccessibleNodes = []model.CatalogNode{}
		status := "unavailable"
		if p.BillingMode == "payg" && g.Enabled && !restricted {
			status = "sync_pending"
			if covered := coverage[p.GroupID]; covered != "" {
				status = covered
			} else {
				found, available, healthy, compatible := false, false, false, false
				for _, a := range accounts {
					if a.GroupID != p.GroupID {
						continue
					}
					found = true
					if s.master != nil && s.master.Connected(a.NodeID) && paygCapabilities(s.master.Capabilities(a.NodeID)) {
						compatible = true
					}
					funded := a.Funded(s.now())
					available = available || funded
					if funded && s.master != nil && s.master.paygNodeReady(a.NodeID) {
						healthy = true
					}
				}
				if found && !available {
					status = "insufficient_balance"
				} else if healthy {
					status = "available"
				} else if found && !compatible {
					status = "unavailable"
				} else if !found && out.Balance.MinorInt64() < p.PAYGPriceTXBMinor {
					status = "insufficient_balance"
				}
			}
		}
		out.Squads = append(out.Squads, PAYGSquad{Squad: p, Status: status, TrafficBytes: strconv.FormatInt(u.TrafficBytes, 10), Spent: model.TXBMoney(u.SpentMinor)})
		total.Add(total, big.NewInt(u.TrafficBytes))
		if spent > math.MaxInt64-u.SpentMinor {
			return out, ErrInvalid
		}
		spent += u.SpentMinor
	}
	out.TrafficBytes = total.String()
	out.Spent = model.TXBMoney(spent)
	_, err = s.queueMissingPAYG(ctx, user, products, accounts, groups)
	return out, err
}

func (m *Master) paygNodeReady(node string) bool {
	m.mu.RLock()
	peer := m.peers[node]
	live := m.live[node]
	m.mu.RUnlock()
	if peer == nil {
		return false
	}
	peer.mu.Lock()
	ready := peer.paygReady
	peer.mu.Unlock()
	return ready && freshSnapshot(live, m.service.now(), 5) && live.report.Ready && live.report.Synchronized
}
