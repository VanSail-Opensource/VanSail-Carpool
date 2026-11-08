package xnode

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

const masterHourlyAgent = "11111111111111111111111111111111"
const masterHourlyBoot = "22222222222222222222222222222222"
const masterHourlySession = "33333333333333333333333333333333"

type hourlySocketRepository struct {
	reportSocketRepository
	progressEntered chan struct{}
	trafficEntered  chan struct{}
	historyReady    bool
}

func (r *hourlySocketRepository) HourlyTrafficHistoryReady() bool { return r.historyReady }

func (*hourlySocketRepository) RegisterXNodeTrafficStream(context.Context, string, string, string) error {
	return nil
}
func (*hourlySocketRepository) XNodeStreamSequence(context.Context, string, string) (uint64, bool, error) {
	return 0, false, nil
}
func (*hourlySocketRepository) SetXNodeStreamRate(context.Context, string, string, string, uint64, int64) error {
	return nil
}

func (r *hourlySocketRepository) RecordHourlyCreditProgressBatch(ctx context.Context, _, _ string, _ []model.HourlyCreditProgress) ([]model.HourlyCreditReservation, error) {
	if r.progressEntered != nil {
		select {
		case r.progressEntered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, nil
}
func (*hourlySocketRepository) ApplyHourlyCreditProof(context.Context, string, string, model.HourlyCreditProof) (model.HourlyCreditReservation, error) {
	return model.HourlyCreditReservation{}, nil
}
func (r *hourlySocketRepository) RecordXNodeTraffic(context.Context, string, string, uint64, string, wire.TrafficReport) error {
	if r.trafficEntered != nil {
		select {
		case r.trafficEntered <- struct{}{}:
		default:
		}
	}
	return nil
}

type hourlyPolicySocketRepository struct {
	*hourlySocketRepository
	policies  []HourlyAccountPolicy
	policyErr error
}

func (r *hourlyPolicySocketRepository) HourlyEnforcementPolicies(context.Context, time.Time) ([]HourlyAccountPolicy, error) {
	return r.policies, r.policyErr
}

func masterHourlyFixture(t *testing.T, repo Repository) (*Master, *httptest.Server, string) {
	t.Helper()
	token := strings.Repeat("a", 43)
	ctx, cancel := context.WithCancel(context.Background())
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	m.ctx = ctx
	server := httptest.NewTLSServer(m)
	t.Cleanup(func() { cancel(); server.Close(); m.Wait() })
	return m, server, token
}

func masterHourlyRepo() *hourlySocketRepository {
	return &hourlySocketRepository{reportSocketRepository: reportSocketRepository{token: sha256.Sum256([]byte(strings.Repeat("a", 43)))}, historyReady: true}
}

func masterHourlySocket(t *testing.T, server *httptest.Server, token, session, format string, capable bool) (*websocket.Conn, *http.Response) {
	t.Helper()
	return masterHourlySocketWithStream(t, server, token, session, format, capable, true)
}

func masterHourlySocketWithStream(t *testing.T, server *httptest.Server, token, session, format string, capable, stream bool) (*websocket.Conn, *http.Response) {
	t.Helper()
	headers := http.Header{"Authorization": []string{"Bearer " + token}}
	headers.Set(wire.TrafficStreamHeader, wire.TrafficStreamFormat)
	if format != "" {
		headers.Set(wire.HourlyHeader, format)
		headers.Set(wire.HourlySessionHeader, session)
	}
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, Subprotocols: []string{"x-node.v1"}}
	socket, response, err := dialer.Dial("wss"+strings.TrimPrefix(server.URL, "https"), headers)
	if err != nil {
		t.Fatal(err)
	}
	caps := []string{}
	if stream {
		caps = append(caps, wire.TrafficStreamCapability)
	}
	if capable {
		caps = append(caps, wire.HourlyCapability)
	}
	if err = socket.WriteJSON(wire.New("agent.hello", "", 0, wire.AgentHello{AgentID: masterHourlyAgent, BootID: masterHourlyBoot, Capabilities: caps})); err != nil {
		socket.Close()
		t.Fatal(err)
	}
	return socket, response
}

func waitMasterHourlyPeer(t *testing.T, m *Master) *agentPeer {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		m.mu.RLock()
		peer := m.peers["node"]
		m.mu.RUnlock()
		if peer != nil {
			return peer
		}
		select {
		case <-timer.C:
			t.Fatal("hourly peer did not attach")
		case <-ticker.C:
		}
	}
}
