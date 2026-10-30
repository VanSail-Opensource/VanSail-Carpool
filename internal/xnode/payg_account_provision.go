package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

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
			return s.reopenPurgedPAYG(ctx, a, price)
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
