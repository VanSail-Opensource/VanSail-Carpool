package xnode

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) queueMissingPAYG(ctx context.Context, user string, products []model.SquadProduct, accounts []PAYGAccount, groups []Group) (bool, error) {
	r, ok := s.paygRepository()
	if !ok {
		return false, nil
	}
	choices, err := r.XNodePAYGPreferences(ctx, user)
	if err != nil {
		return false, err
	}
	known := map[string]bool{}
	for _, a := range accounts {
		known[a.GroupID+"\x00"+a.NodeID] = true
	}
	payg := map[string]bool{}
	for _, p := range products {
		payg[p.GroupID] = p.PAYGAvailable()
	}
	for _, g := range groups {
		if !choices.Allows(g.ID) || !g.Enabled || !payg[g.ID] {
			continue
		}
		for _, b := range g.Bindings {
			if known[g.ID+"\x00"+b.NodeID] {
				continue
			}
			node, err := s.repo.XNodeNode(ctx, b.NodeID)
			if err != nil {
				return false, err
			}
			if node.Enabled {
				raw, err := json.Marshal(map[string]string{"userId": user})
				if err != nil {
					return false, err
				}
				return true, r.EnqueueOutbox(ctx, PAYGJob, string(raw), s.now())
			}
		}
	}
	return false, nil
}

// DIY may be the first view opened after a small top-up. Request durable local
// provisioning independently of the balance needed to authorize a GB block.
func (s *Service) requestDIYPAYG(ctx context.Context, user string) (bool, error) {
	r, ok := s.paygRepository()
	if !ok {
		return false, nil
	}
	member, err := s.repo.UserByID(ctx, user)
	if err != nil || member.OnboardingState != "complete" {
		return false, err
	}
	balance, err := r.Balance(ctx, user)
	if err != nil || balance.MinorInt64() <= 0 {
		return false, err
	}
	products, err := r.ListSquadProductOverrides(ctx, false)
	if err != nil {
		return false, err
	}
	accounts, err := r.XNodePAYGAccounts(ctx, user, "")
	if err != nil {
		return false, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return false, err
	}
	return s.queueMissingPAYG(ctx, user, products, accounts, groups)
}
