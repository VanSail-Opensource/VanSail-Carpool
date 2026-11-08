package xnode

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyTLSHandshakeRequiresRepositoryOfferAndHelloCapability(t *testing.T) {
	for _, entry := range []struct {
		name              string
		repository, hello bool
		format, session   string
		echo, capable     bool
	}{
		{"capable", true, true, wire.HourlyFormat, masterHourlySession, true, true},
		{"header-only", true, false, wire.HourlyFormat, masterHourlySession, true, false},
		{"hello-only", true, true, "", "", false, false},
		{"bad-version", true, true, "v2", masterHourlySession, false, false},
		{"bad-session", true, true, wire.HourlyFormat, "not-a-session", false, false},
		{"no-credit-repository", false, true, wire.HourlyFormat, masterHourlySession, false, false},
	} {
		t.Run(entry.name, func(t *testing.T) {
			var repo Repository = masterHourlyRepo()
			if !entry.repository {
				repo = reportSocketRepository{token: sha256.Sum256([]byte(strings.Repeat("a", 43)))}
			}
			m, server, token := masterHourlyFixture(t, repo)
			socket, response := masterHourlySocket(t, server, token, entry.session, entry.format, entry.hello)
			defer socket.Close()
			if (response.Header.Get(wire.HourlyAccountingHeader) == wire.HourlyAccountingFormat) != entry.repository {
				t.Fatal("historical accounting support depended on live admission offer")
			}
			if (response.Header.Get(wire.HourlyHeader) == wire.HourlyFormat) != entry.echo {
				t.Fatal("unsupported hourly upgrade offer was echoed")
			}
			if entry.echo && response.Header.Get(wire.HourlySessionHeader) != entry.session {
				t.Fatal("master changed the offered control session")
			}
			peer := waitMasterHourlyPeer(t, m)
			if m.HourlyCapable("node") != entry.capable || (peer.hourlySession != "") != entry.capable {
				t.Fatal("header or version inferred unsupported hourly capability")
			}
			if entry.capable {
				m.refreshHourlyPolicies(context.Background()) // No commerce policy provider exists yet.
				if _, eligible := m.hourlyPolicy(wire.HourlyOwner{ID: "ordinary-user", Generation: 1}, "node", time.Now()); eligible {
					t.Fatal("transport capability activated a legacy customer")
				}
			} else if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: masterHourlySession}), time.Now()); err == nil {
				t.Fatal("unnegotiated peer entered hourly admission")
			}
		})
	}
}

func TestHourlyOldTLSConnectionCannotKeepCapabilityAfterReplacement(t *testing.T) {
	m, server, token := masterHourlyFixture(t, masterHourlyRepo())
	first, _ := masterHourlySocket(t, server, token, masterHourlySession, wire.HourlyFormat, true)
	defer first.Close()
	old := waitMasterHourlyPeer(t, m)
	freshSession := "44444444444444444444444444444444"
	second, _ := masterHourlySocket(t, server, token, freshSession, wire.HourlyFormat, true)
	defer second.Close()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		m.mu.RLock()
		current := m.peers["node"]
		m.mu.RUnlock()
		if current != nil && current != old {
			if m.currentHourlyPeer(old) || current.hourlySession != freshSession {
				t.Fatal("replaced TLS session retained hourly authority")
			}
			break
		}
		select {
		case <-timer.C:
			t.Fatal("fresh hourly session not installed")
		case <-ticker.C:
		}
	}
}
