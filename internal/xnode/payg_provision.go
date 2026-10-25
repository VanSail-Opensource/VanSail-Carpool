package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strings"
)

func isPAYGIdentity(i Identity) bool { return strings.HasPrefix(i.ServiceName, paygPrefix) }

// ReconcilePAYG is invoked exclusively through the shared durable outbox.
// Tuples remain dormant rather than being deleted on mode/coverage changes.
func (s *Service) ReconcilePAYG(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		UserID string `json:"userId"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.UserID == "" {
		return ErrInvalid
	}
	r, ok := s.paygRepository()
	if !ok {
		return ErrUnavailable
	}
	user, err := s.repo.UserByID(ctx, input.UserID)
	if err != nil || user.OnboardingState != "complete" {
		return err
	}
	choices, err := r.XNodePAYGPreferences(ctx, user.ID)
	if err != nil {
		return err
	}
	products, err := r.ListSquadProducts(ctx, false)
	if err != nil {
		return err
	}
	prices := map[string]int64{}
	for _, p := range products {
		if p.PAYGAvailable() && choices.Allows(p.GroupID) {
			prices[p.GroupID] = p.PAYGPriceTXBMinor
		}
	}

	if len(prices) > 0 {
		if _, err = s.EnsureIdentity(ctx, user); err != nil {
			return err
		}
	}
	accounts, err := r.XNodePAYGAccounts(ctx, user.ID, "")
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, a := range accounts {
		known[a.GroupID+"\x00"+a.NodeID] = true
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return err
	}
	for _, g := range groups {
		price := prices[g.ID]
		if !g.Enabled || price <= 0 {
			continue
		}
		seen := map[string]bool{}
		for _, b := range g.Bindings {
			if seen[b.NodeID] || known[g.ID+"\x00"+b.NodeID] {
				continue
			}
			seen[b.NodeID] = true
			node, e := s.repo.XNodeNode(ctx, b.NodeID)
			if e != nil {
				return e
			}
			if !node.Enabled {
				continue
			}
			if err = s.ensurePAYGAccount(ctx, user.ID, g.ID, b.NodeID, price); err != nil {
				return err
			}
		}
	}
	if s.master != nil {
		for _, account := range accounts {
			s.master.mu.RLock()
			peer := s.master.peers[account.NodeID]
			s.master.mu.RUnlock()
			if peer != nil {
				s.master.requestSync(peer)
			}
		}
	}
	return nil
}

func (s *Service) paygCoverage(ctx context.Context, user string) (map[string]string, error) {
	purchases, err := s.repo.ListPurchases(ctx, user)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, p := range purchases {
		if p.Status != "queued" && p.Status != "active" && p.Status != "activating" {
			continue
		}
		if !p.ValidUntil.After(s.now()) {
			continue
		}
		status := "combo_covered"
		if p.Status == "queued" {
			status = "queued_combo"
		}
		for _, id := range p.SquadUUIDs {
			if out[id] != "combo_covered" {
				out[id] = status
			}
		}
	}
	return out, nil
}
