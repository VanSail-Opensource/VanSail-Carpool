package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPAYGBlockBoundaryPreservesAuthorization(t *testing.T) {
	service, base, peer, master := compileCacheFixture(t)
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	a := PAYGAccount{ProxyUserID: 8, NodeID: peer.nodeID, PeriodID: "payg-continuous", BalanceMinor: 200, UnitPriceMinor: 100, CreditStartedAt: now}
	r := &paygLeaseRepository{cacheTestRepository: base}
	service.repo, peer.paygReady = r, true
	master.live[peer.nodeID] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	config := wire.Config{Users: []wire.User{{ID: 8, UUID: "stable-credential", AccountingPeriod: a.PeriodID}}}
	caps := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"}
	for _, credit := range []int64{1, 0, PAYGGB - 1} {
		a.CreditBytes = credit
		r.accounts = []PAYGAccount{a}
		got, err := master.overlayPAYG(context.Background(), peer, config, caps)
		if err != nil || len(got.Users) != 1 || got.Users[0].UUID != "stable-credential" || got.Users[0].AccountingPeriod != a.PeriodID || got.Users[0].ValidUntil == nil || !got.Users[0].ValidUntil.After(now) {
			t.Fatalf("credit %d interrupted funded authorization: %+v, %v", credit, got.Users, err)
		}
	}
	// Compensate the prepaid cap in the same clock as the renewable lease.
	a.BalanceMinor, a.CreditBytes = 0, 1
	a.CreditStartedAt = now.Add(-PAYGWindow + 10*time.Second)
	r.accounts = []PAYGAccount{a}
	master.live[peer.nodeID] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true, Time: now.Add(85 * time.Second)}}
	got, err := master.overlayPAYG(context.Background(), peer, config, caps)
	if err != nil || len(got.Users) != 1 || !got.Users[0].ValidUntil.Equal(now.Add(95*time.Second)) {
		t.Fatalf("credit expiry used a different clock: %+v, %v", got.Users, err)
	}
}

type paygObservationFailure struct {
	*paygLeaseRepository
	syncAttempted bool
	failure       error
}

func (r *paygObservationFailure) XNodePrincipalOwners(context.Context) (map[int]string, error) {
	return map[int]string{}, nil
}
func (r *paygObservationFailure) XNodeNodes(context.Context) ([]Node, error) {
	return []Node{r.node}, nil
}
func (r *paygObservationFailure) XNodePAYGConnectedAccounts(context.Context, string, []int64) ([]PAYGAccount, error) {
	return nil, r.failure
}

func TestPAYGObservationFailureCannotSkipNodeSynchronization(t *testing.T) {
	service, base, peer, master := compileCacheFixture(t)
	r := &paygObservationFailure{paygLeaseRepository: &paygLeaseRepository{cacheTestRepository: base}, failure: errors.New("observation unavailable")}
	// Stop compilation before transport; the node read proves sync was attempted.
	base.failure = errors.New("compile deferred")
	base.beforeRead = func() { r.syncAttempted = true }
	service.repo, peer.paygReady = r, true
	master.peers[peer.nodeID] = peer
	master.live[peer.nodeID] = liveSnapshot{receivedAt: time.Now(), report: wire.TelemetryReport{Synchronized: true, Users: wire.UserMetrics{Connections: map[int]int{8: 1}}}}
	if err := master.reconcile(context.Background()); !errors.Is(err, r.failure) || !r.syncAttempted {
		t.Fatalf("observation skipped synchronization: attempted=%t error=%v", r.syncAttempted, err)
	}
}
