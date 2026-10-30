package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Service) reopenPurgedPAYG(ctx context.Context, a PAYGAccount, price int64) error {
	identity, err := s.repo.XNodeIdentity(ctx, a.ProxyUserID)
	if err != nil {
		return err
	}
	if identity.CredentialsCipher != "" {
		return nil
	}
	user, err := s.repo.UserByID(ctx, a.UserID)
	if err != nil {
		return err
	}
	if user.AccountStatus != "active" || user.AccountGeneration < 1 {
		return ErrConflict
	}
	r, ok := s.repo.(interface {
		ReopenPurgedPAYG(context.Context, Identity, Period, string, int64, int64) error
	})
	if !ok {
		return ErrUnavailable
	}
	credentials, err := NewCredentials()
	if err != nil {
		return err
	}
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return err
	}
	periodID, err := ids.New()
	if err != nil {
		return err
	}
	period := Period{ID: "payg-" + periodID, ProxyUserID: a.ProxyUserID, StartedAt: s.now().UTC(), Strategy: "PAYG", Status: "open"}
	return r.ReopenPurgedPAYG(ctx, identity, period, a.UserID, user.AccountGeneration, price)
}
