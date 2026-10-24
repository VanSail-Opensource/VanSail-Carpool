package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

type paygLeaseRepository struct {
	*cacheTestRepository
	accounts []PAYGAccount
	enqueues int
}

func (r *paygLeaseRepository) XNodeIdentityByService(context.Context, string) (Identity, error) {
	return Identity{}, ErrNotFound
}

func (r *paygLeaseRepository) XNodePAYGAccounts(context.Context, string, string) ([]PAYGAccount, error) {
	return r.accounts, nil
}
func (r *paygLeaseRepository) XNodePAYGAccount(context.Context, int64) (PAYGAccount, error) {
	return r.accounts[0], nil
}
func (r *paygLeaseRepository) SaveXNodePAYGAccount(context.Context, PAYGAccount) error { return nil }
func (r *paygLeaseRepository) ObserveXNodePAYGConnections(context.Context, string, []int64, time.Time) error {
	panic("lease refresh must not bill")
}
func (r *paygLeaseRepository) XNodePAYGUsage(context.Context, string, time.Time, time.Time) ([]PAYGUsage, error) {
	return nil, nil
}
func (r *paygLeaseRepository) ListSquadProducts(context.Context, bool) ([]model.SquadProduct, error) {
	return nil, nil
}
func (r *paygLeaseRepository) Balance(context.Context, string) (model.Money, error) {
	return model.TXBMoney(0), nil
}
func (r *paygLeaseRepository) EnqueueOutbox(context.Context, string, string, time.Time) error {
	r.enqueues++
	return nil
}

func TestPAYGLeasesExpireIndependentlyAndPreserveNativeCandidate(t *testing.T) {
	service, base, peer, master := compileCacheFixture(t)
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	r := &paygLeaseRepository{cacheTestRepository: base, accounts: []PAYGAccount{{ProxyUserID: 4503599627370496, NodeID: "node", PurchasedBytes: PAYGGB, UsedBytes: 1, UnitPriceMinor: 100}}}
	service.repo = r
	peer.paygReady = true
	master.live["node"] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	config := wire.Config{Users: []wire.User{{ID: 7}, {ID: int(r.accounts[0].ProxyUserID), AccountingPeriod: "payg-test"}}}
	caps := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"}
	first, err := master.overlayPAYG(context.Background(), peer, config, caps)
	if err != nil || len(first.Users) != 2 || first.Users[1].ValidUntil == nil {
		t.Fatal(first, err)
	}
	until := *first.Users[1].ValidUntil
	if until.Sub(now) > 60*time.Second || until.Sub(now) < 40*time.Second || config.Users[1].ValidUntil != nil {
		t.Fatal("lease mutated cached native input", until, config)
	}
	key := peer.paygOverlayKey
	now = now.Add(21 * time.Second)
	master.live["node"] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	next, err := master.overlayPAYG(context.Background(), peer, config, caps)
	if err != nil || !next.Users[1].ValidUntil.After(until) || peer.paygOverlayKey == key || base.reads != 0 {
		t.Fatal("lease refresh compiled native state", next, err, base.reads)
	}
	r.accounts[0].BalanceMinor = -1
	blocked, err := master.overlayPAYG(context.Background(), peer, config, caps)
	if err != nil || len(blocked.Users) != 1 || blocked.Users[0].ID != 7 {
		t.Fatal("debt disturbed combo or retained PAYG", blocked, err)
	}
	r.accounts[0].BalanceMinor = 0
	for _, capabilities := range [][]string{nil, caps} {
		if capabilities != nil {
			master.live["node"] = liveSnapshot{receivedAt: now.Add(-time.Minute), report: wire.TelemetryReport{Ready: true, Synchronized: true}}
		}
		blocked, err = master.overlayPAYG(context.Background(), peer, config, capabilities)
		if err != nil || len(blocked.Users) != 1 {
			t.Fatal("unsupported/stale node renewed PAYG", blocked, err)
		}
	}
}

func TestWhitelistCannotEnableWithoutActiveComboQuota(t *testing.T) {
	if _, err := normalizeWhitelist(WhitelistPolicy{}, WhitelistPolicy{Enabled: true}, 0); err == nil {
		t.Fatal("PAYG-only whitelist enabled")
	}
}
