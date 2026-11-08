package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// PAYGPreparation binds uncharged native provisioning to one account generation.
type PAYGPreparation struct {
	OwnerID, GroupID, NodeID, ServiceName, PeriodID, CredentialsCipher string
	AccountGeneration, UnitPriceMinor                                  int64
}

type paygPreparationRepository interface {
	PrepareXNodePAYGAccount(context.Context, PAYGPreparation) (PAYGAccount, error)
}

// PAYGAccountServiceName preserves the existing deterministic private identity.
func PAYGAccountServiceName(owner, group, node string) string {
	hash := sha256.Sum256([]byte(owner + "\x00" + group + "\x00" + node))
	return paygPrefix + hex.EncodeToString(hash[:20])
}

// The production store persists principal/period/association together. Encryption
// and entropy run before its lifecycle transaction, without a service-wide lock.
func (s *Service) preparePAYGAccount(ctx context.Context, repo paygPreparationRepository, owner, group, node string, price int64) error {
	accountsRepo, ok := s.paygRepository()
	if !ok {
		return ErrUnavailable
	}
	accounts, err := accountsRepo.XNodePAYGAccounts(ctx, owner, node)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		if account.GroupID == group {
			return s.reopenPurgedPAYG(ctx, account, price)
		}
	}
	user, err := s.repo.UserByID(ctx, owner)
	if err != nil {
		return err
	}
	if user.AccountStatus != "active" {
		return ErrConflict
	}
	name := PAYGAccountServiceName(owner, group, node)
	credentials, err := NewCredentials()
	if err != nil {
		return err
	}
	cipher, err := sealCredentials(s.vault, Identity{ServiceName: name}, credentials)
	if err != nil {
		return err
	}
	period, err := ids.New()
	if err != nil {
		return err
	}
	account, err := repo.PrepareXNodePAYGAccount(ctx, PAYGPreparation{OwnerID: owner, GroupID: group, NodeID: node, ServiceName: name, PeriodID: "payg-" + period, CredentialsCipher: cipher, AccountGeneration: user.AccountGeneration, UnitPriceMinor: price})
	if err != nil {
		return err
	}
	return s.reopenPurgedPAYG(ctx, account, price)
}
