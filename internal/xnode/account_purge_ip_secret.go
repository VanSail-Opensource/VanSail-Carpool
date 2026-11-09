package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type connectionIPOwnerRepository interface {
	ConnectionIPOwnerAliases(context.Context, string) ([]secret.ConnectionIPOwnerAlias, error)
}

func (s *Service) openAccountPurgeIPTarget(ctx context.Context, owner string, block connections.IPBlockRecord) ([]byte, error) {
	if block.UserID != owner || s.vault == nil {
		return nil, secret.ErrConnectionIPTarget
	}
	var lookup secret.ConnectionIPOwnerLookup
	if repo, ok := s.repo.(connectionIPOwnerRepository); ok {
		lookup = repo.ConnectionIPOwnerAliases
	}
	return s.vault.OpenConnectionIPTarget(ctx, secret.ConnectionIPTarget{
		OwnerID: owner, NodeID: block.NodeUUID, Digest: block.IPDigest, Ciphertext: block.SealedIP,
	}, lookup)
}
