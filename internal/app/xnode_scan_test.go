package app

import (
	"context"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type scanIdentityRepository struct{ xnode.Repository }

func (scanIdentityRepository) XNodeIdentityByUser(_ context.Context, userID string) (xnode.Identity, error) {
	return xnode.Identity{UserID: userID, ProxyUserID: 42}, nil
}

func TestNativeScanAllocatesValidNonceWithoutLegacyIdentity(t *testing.T) {
	provider := proxyProvider{native: xnode.NewService(scanIdentityRepository{}, nil, nil, nil, nil)}
	job, err := provider.RequestNativeConnectionScan(context.Background(), "member")
	if err != nil || !strings.HasPrefix(job, "xna:42:") || len(strings.Split(job, ":")[2]) < 22 {
		t.Fatalf("native scan start failed: %q, %v", job, err)
	}
}
