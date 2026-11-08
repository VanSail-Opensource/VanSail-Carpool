package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func paygPreparationFixture(t *testing.T) (*Store, xnode.PAYGPreparation) {
	t.Helper()
	s, parent, _ := xnodeTrafficFixture(t)
	if _, err := s.SaveSquadProduct(context.Background(), SquadProductInput{GroupID: "atomic-payg", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	input := xnode.PAYGPreparation{OwnerID: parent.UserID, GroupID: "atomic-payg", NodeID: "node", AccountGeneration: 0, PeriodID: "payg-atomic", CredentialsCipher: "new-atomic-cipher", UnitPriceMinor: 100}
	input.ServiceName = xnode.PAYGAccountServiceName(input.OwnerID, input.GroupID, input.NodeID)
	return s, input
}

func waitNativeIdentityQueryBlocked(t *testing.T, s *Store, ctx context.Context, lock string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid()
 AND wait_event_type='Lock' AND strpos(query,'account_status,account_generation')>0 AND strpos(query,$1)>0)`, lock).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("native credential writer did not wait on lifecycle lock")
}
