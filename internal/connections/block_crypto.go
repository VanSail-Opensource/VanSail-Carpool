package connections

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type connectionIPOwnerRepository interface {
	ConnectionIPOwnerAliases(context.Context, string) ([]secret.ConnectionIPOwnerAlias, error)
}

func openIPBlockTarget(ctx context.Context, repo any, box SecretBox, block IPBlockRecord) ([]byte, error) {
	var lookup secret.ConnectionIPOwnerLookup
	if ownerRepo, ok := repo.(connectionIPOwnerRepository); ok {
		lookup = ownerRepo.ConnectionIPOwnerAliases
	}
	return secret.OpenConnectionIPTarget(ctx, secret.ConnectionIPTarget{
		OwnerID: block.UserID, NodeID: block.NodeUUID, Digest: block.IPDigest, Ciphertext: block.SealedIP,
	}, box.Open, lookup)
}
