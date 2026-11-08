package xnode_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

const hourlyProductionSession = "33333333333333333333333333333333"

type hourlyProductionRepository struct {
	*database.Store
	mu               sync.Mutex
	policies         []xnode.HourlyAccountPolicy
	loseReserveReply bool
	beforeReserve    func(context.Context, model.HourlyCreditRequest) error
	scans            atomic.Int64
}

func (r *hourlyProductionRepository) UnfinalizedHourlyCredits(ctx context.Context, node, agent string, after int64, limit int) ([]model.HourlyCreditReservation, error) {
	rows, err := r.Store.UnfinalizedHourlyCredits(ctx, node, agent, after, limit)
	r.scans.Add(1)
	return rows, err
}

func (r *hourlyProductionRepository) HourlyEnforcementPolicies(context.Context, time.Time) ([]xnode.HourlyAccountPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]xnode.HourlyAccountPolicy(nil), r.policies...), nil
}

func (r *hourlyProductionRepository) ReserveHourlyCredit(ctx context.Context, request model.HourlyCreditRequest) (model.HourlyCreditReservation, error) {
	r.mu.Lock()
	before := r.beforeReserve
	r.beforeReserve = nil
	r.mu.Unlock()
	if before != nil {
		if err := before(ctx, request); err != nil {
			return model.HourlyCreditReservation{}, err
		}
	}
	value, err := r.Store.ReserveHourlyCredit(ctx, request)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil && r.loseReserveReply {
		r.loseReserveReply = false
		return model.HourlyCreditReservation{}, xnode.ErrUnavailable
	}
	return value, err
}

type hourlyProductionFixture struct {
	f      fixture
	repo   *hourlyProductionRepository
	master *xnode.Master
	server *httptest.Server
	token  string
}

func newHourlyProductionFixture(t *testing.T, allowance int64) hourlyProductionFixture {
	t.Helper()
	f := controlFixture(t, false)
	now := time.Now()
	repo := &hourlyProductionRepository{Store: f.store, policies: []xnode.HourlyAccountPolicy{{OwnerID: f.user.ID, AccountGeneration: f.user.AccountGeneration, PaidThrough: now.Add(time.Hour), RateBytesPerSecond: 1000, DailyAllowanceBytes: allowance, NodePaidThrough: map[string]time.Time{f.node.ID: now.Add(time.Hour)}}}}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := xnode.NewService(repo, &fixtureSettings{}, vault, nil, nil)
	m := xnode.NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	if err = m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(m)
	t.Cleanup(func() {
		cancel()
		stop, finish := context.WithTimeout(context.Background(), 5*time.Second)
		defer finish()
		if err := m.Shutdown(stop); err != nil {
			t.Error(err)
		}
		server.Close()
	})
	_, token, err := f.service.Enrollment(context.Background(), f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	return hourlyProductionFixture{f, repo, m, server, token}
}

func (f hourlyProductionFixture) connect(t *testing.T, session string) *websocket.Conn {
	t.Helper()
	headers := http.Header{"Authorization": []string{"Bearer " + f.token}, wire.HourlyHeader: []string{wire.HourlyFormat}, wire.HourlySessionHeader: []string{session}}
	headers.Set(wire.TrafficStreamHeader, wire.TrafficStreamFormat)
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, Subprotocols: []string{"x-node.v1"}}
	socket, response, err := dialer.Dial("wss"+strings.TrimPrefix(f.server.URL, "https"), headers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	if response.Header.Get(wire.TrafficStreamHeader) != wire.TrafficStreamFormat || response.Header.Get(wire.HourlyHeader) != wire.HourlyFormat || response.Header.Get(wire.HourlySessionHeader) != session || response.Header.Get(wire.HourlyAccountingHeader) != wire.HourlyAccountingFormat {
		t.Fatal("credit fixture did not negotiate boot-stream accounting and the exact hourly session")
	}
	if err = socket.WriteJSON(wire.New("agent.hello", "", 0, wire.AgentHello{AgentID: fixtureAgent, BootID: fixtureBoot, Capabilities: []string{wire.HourlyCapability, wire.TrafficStreamCapability}})); err != nil {
		t.Fatal(err)
	}
	waitHourlyProduction(t, func() bool { return f.master.HourlyCreditAdmissionReady(f.f.node.ID) })
	return socket
}

func (f hourlyProductionFixture) demand(t *testing.T, socket *websocket.Conn, session string) {
	t.Helper()
	demand := wire.HourlyDemand{Owner: wire.HourlyOwner{ID: f.f.user.ID, Generation: uint64(f.f.user.AccountGeneration)}, Nonce: strings.Repeat("a", 32), LifetimeMillis: 10000, PendingBytes: 100, MinAtomicBytes: 1}
	if err := socket.WriteJSON(wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: session, Demands: []wire.HourlyDemand{demand}})); err != nil {
		t.Fatal(err)
	}
}

func readHourlyProductionCommand(t *testing.T, socket *websocket.Conn, name string, args any) wire.Message {
	t.Helper()
	if err := socket.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		message := readFrame(t, socket, "command")
		var command struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
		}
		if wire.Decode(message.Data, &command) != nil {
			t.Fatal("invalid queued command")
		}
		if command.Name == name {
			if wire.Decode(command.Args, args) != nil {
				t.Fatal("invalid command arguments")
			}
			return message
		}
		if strings.HasPrefix(command.Name, "hourly.credit.") {
			t.Fatalf("unexpected credit command %s while waiting for %s", command.Name, name)
		}
	}
}

func finishHourlyProductionCommand(t *testing.T, socket *websocket.Conn, message wire.Message, name string, data any) {
	t.Helper()
	if err := socket.WriteJSON(wire.New("command.result", message.ID, 0, map[string]any{"id": message.ID, "command": name, "status": "completed", "finished": time.Now(), "data": data})); err != nil {
		t.Fatal(err)
	}
}

func waitHourlyProduction(t *testing.T, ready func() bool) {
	t.Helper()
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for !ready() {
		select {
		case <-timer.C:
			t.Fatal("hourly credit condition did not settle")
		case <-ticker.C:
		}
	}
}
