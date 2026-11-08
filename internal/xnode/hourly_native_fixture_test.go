package xnode_test

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

type hourlyNativeAgent struct {
	mu            sync.Mutex
	config        wire.Config
	revision      uint64
	writeMu       sync.Mutex
	socket        *websocket.Conn
	done          chan struct{}
	telemetryDone chan struct{}
}

type hourlyNativePolicy struct{}

func (hourlyNativePolicy) HourlyPolicy(context.Context) (model.HourlyPolicySnapshot, error) {
	return model.HourlyPolicySnapshot{Version: strings.Repeat("a", 64), Policy: model.HourlyPolicy{SalesEnabled: false, DailyAllowanceBytes: 1000, ReducedRateBytesPerSecond: 1000, ActivationHours: 168, CooldownSeconds: 86400}}, nil
}

func hourlyNativeFixture(t *testing.T) fixture {
	t.Helper()
	f := controlFixture(t, false)
	stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.master.Shutdown(stop); err != nil {
		t.Fatal(err)
	}
	// Install providers before the replacement master's workers start; changing
	// a live provider pointer would race the independent policy reader.
	f.service.SetHourlyPolicyProvider(hourlyNativePolicy{})
	f.master = xnode.NewMaster(f.service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, finish := context.WithCancel(context.Background())
	if err := f.master.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		finish()
		ctx, end := context.WithTimeout(context.Background(), 5*time.Second)
		defer end()
		if err := f.master.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return f
}

func connectHourlyNativeAgent(t *testing.T, f fixture) *hourlyNativeAgent {
	t.Helper()
	server := httptest.NewTLSServer(f.master)
	t.Cleanup(server.Close)
	_, token, err := f.service.Enrollment(context.Background(), f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	headers := http.Header{"Authorization": []string{"Bearer " + token}, wire.HourlyHeader: []string{wire.HourlyFormat}, wire.HourlySessionHeader: []string{hourlyProductionSession}, wire.TrafficStreamHeader: []string{wire.TrafficStreamFormat}}
	dialer := websocket.Dialer{Subprotocols: []string{"x-node.v1"}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	socket, response, err := dialer.Dial("wss"+strings.TrimPrefix(server.URL, "https"), headers)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get(wire.HourlyHeader) != wire.HourlyFormat {
		t.Fatal("native fixture did not negotiate hourly")
	}
	a := &hourlyNativeAgent{socket: socket, done: make(chan struct{}), telemetryDone: make(chan struct{})}
	caps := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "sampled_activity_v1", "config_validation_v1", "transit_credentials_v1", "authorization_expiry", "member_outbound_guards", wire.HourlyCapability, wire.TrafficStreamCapability}
	if err = socket.WriteJSON(wire.New("agent.hello", "", 0, wire.AgentHello{AgentID: fixtureAgent, BootID: fixtureBoot, Version: "fixture", Capabilities: caps})); err != nil {
		t.Fatal(err)
	}
	go a.run()
	go a.telemetry()
	t.Cleanup(func() { _ = socket.Close(); <-a.done; <-a.telemetryDone })
	waitHourlyProduction(t, func() bool { return f.master.HourlyCreditAdmissionReady(f.node.ID) })
	if err = f.master.Sync(context.Background(), f.node.ID); err != nil {
		t.Fatal(err)
	}
	return a
}

func seedNativeHourlyPreparation(t *testing.T, f fixture) xnode.HourlyPrincipal {
	t.Helper()
	if _, err := f.store.DB().Exec(`INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES('native-enrollment',$1,$2,$3,'native-key',$4,'preparing',clock_timestamp(),clock_timestamp())`, f.user.ID, f.user.AccountGeneration, f.group.ID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	p, err := f.service.PrepareHourlyPrincipal(context.Background(), "native-enrollment")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func waitHourlyNativeReadiness(t *testing.T, ctx context.Context, f fixture) {
	t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		ready, err := f.service.HourlySquadReadiness(ctx, f.group.ID, f.user.ID)
		if err == nil && ready.Ready {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-timer.C:
			t.Fatal("native admission did not leave startup quiet", ready, err)
		case <-tick.C:
		}
	}
}
