package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
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
	products, err := r.ListSquadProducts(ctx, false)
	if err != nil {
		return err
	}
	prices := map[string]int64{}
	for _, p := range products {
		if p.BillingMode == "payg" {
			prices[p.GroupID] = p.PAYGPriceTXBMinor
		}
	}
	if len(prices) == 0 {
		return nil
	}
	if _, err = s.EnsureIdentity(ctx, user); err != nil {
		return err
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
	return nil
}

func (s *Service) ensurePAYGAccount(ctx context.Context, user, group, node string, price int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, _ := s.paygRepository()
	accounts, err := r.XNodePAYGAccounts(ctx, user, node)
	if err != nil {
		return err
	}
	for _, a := range accounts {
		if a.GroupID == group {
			return nil
		}
	}
	hash := sha256.Sum256([]byte(user + "\x00" + group + "\x00" + node))
	name := paygPrefix + hex.EncodeToString(hash[:20])
	i, err := r.XNodeIdentityByService(ctx, name)
	if err != nil && !isMissing(err) {
		return err
	}
	if isMissing(err) {
		i = Identity{ServiceName: name}
	}
	if i.UserID != "" || len(i.SubscriptionHash) > 0 {
		return ErrConflict
	}
	if i.ProxyUserID == 0 {
		c, e := NewCredentials()
		if e != nil {
			return e
		}
		i.CredentialsCipher, e = sealCredentials(s.vault, i, c)
		if e != nil {
			return e
		}
		i, e = s.repo.SaveXNodeIdentity(ctx, i)
		if e != nil {
			return e
		}
	}
	period := i.CurrentPeriodID
	if period == "" {
		period, err = ids.New()
		if err != nil {
			return err
		}
		period = "payg-" + period
		if err = s.repo.SaveXNodePeriod(ctx, Period{ID: period, ProxyUserID: i.ProxyUserID, StartedAt: s.now().UTC(), Strategy: "PAYG", Status: "open"}); err != nil {
			return err
		}
	}
	return r.SaveXNodePAYGAccount(ctx, PAYGAccount{ProxyUserID: i.ProxyUserID, UserID: user, GroupID: group, NodeID: node, PeriodID: period, UnitPriceMinor: price})
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
