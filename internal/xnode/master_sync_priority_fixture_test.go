package xnode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/gorilla/websocket"
)

type renewalBeforeRoutingRepository struct {
	*paygLeaseRepository
	routingRepository
	nodes          []Node
	funding        PAYGFunding
	fundingReads   atomic.Int32
	advances       atomic.Int32
	observations   int
	routingCalls   int
	planReads      int
	observationErr error
	waitForExpiry  bool
	priorRenewals  int32
}

func (r *renewalBeforeRoutingRepository) XNodeNodes(ctx context.Context) ([]Node, error) {
	return r.nodes, ctx.Err()
}

func (r *renewalBeforeRoutingRepository) XNodePAYGFunding(ctx context.Context, node string) ([]PAYGFunding, error) {
	r.fundingReads.Add(1)
	funding := r.funding
	funding.NodeID = node
	return []PAYGFunding{funding}, ctx.Err()
}

func (r *renewalBeforeRoutingRepository) AdvanceXNodeRevision(ctx context.Context, _ string, previous uint64) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.advances.Add(1)
	return previous + 1, nil
}

func (r *renewalBeforeRoutingRepository) XNodePAYGConnectedAccounts(context.Context, string, []int64) ([]PAYGAccount, error) {
	r.observations++
	return nil, r.observationErr
}

func (r *renewalBeforeRoutingRepository) WakeXNodeRouting(ctx context.Context, _ time.Time) error {
	r.routingCalls++
	r.priorRenewals = r.advances.Load()
	if r.waitForExpiry {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (*renewalBeforeRoutingRepository) XNodeUnmanagedRoutes(context.Context, time.Time) ([]Customization, error) {
	return nil, nil
}

func (r *renewalBeforeRoutingRepository) XNodeRoutingPlans(context.Context) ([]RoutingPlan, error) {
	r.planReads++
	return nil, nil
}

// A real socket and the production serialized queue make successful publication
// observable without starting the master's background schedulers or a database.
func renewalRoutingSocket(t *testing.T, peer *agentPeer) *websocket.Conn {
	t.Helper()
	sockets := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		socket, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			sockets <- socket
		}
	}))
	t.Cleanup(server.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	peer.socket = <-sockets
	peer.ctx, peer.cancel = context.WithCancel(context.Background())
	t.Cleanup(func() {
		peer.cancel()
		_ = client.Close()
		_ = peer.socket.Close()
	})
	peer.queue, err = upstreamqueue.New(upstreamqueue.Config{Name: "renewal-routing-" + peer.nodeID, Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err = peer.queue.Start(peer.ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := peer.queue.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return client
}
