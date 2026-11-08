package xnode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestUnreadyHourlyHistoryCannotAdvertiseReceiptsOrAdmission(t *testing.T) {
	repo := &hourlyReceiptRepository{masterHourlyRepo()}
	repo.historyReady = false
	m, server, token := masterHourlyFixture(t, repo)
	if m.hourly == nil || m.hourlyCredits == nil {
		t.Fatal("history admission gate removed financial settlement construction")
	}
	request := httptest.NewRequest(http.MethodGet, "/xnode", nil)
	request.Header.Set(wire.TrafficStreamHeader, wire.TrafficStreamFormat)
	request.Header.Set(wire.HourlyHeader, wire.HourlyFormat)
	request.Header.Set(wire.HourlySessionHeader, masterHourlySession)
	response := http.Header{}
	if session := m.hourlyOffer(request, response); session != "" || response.Get(wire.HourlyHeader) != "" || response.Get(wire.HourlyAccountingHeader) != "" {
		t.Fatal("unimplemented history ingestion advertised receipt or live support")
	}
	socket, upgrade := masterHourlySocket(t, server, token, masterHourlySession, wire.HourlyFormat, true)
	defer socket.Close()
	peer := waitMasterHourlyPeer(t, m)
	if upgrade.Header.Get(wire.HourlyHeader) != "" || upgrade.Header.Get(wire.HourlyAccountingHeader) != "" || peer.hourlySession != "" || m.HourlyCapable(peer.nodeID) {
		t.Fatal("hello capability bypassed the missing durable history writer")
	}
}

type hourlyHistoryGateRepository struct {
	*hourlySocketRepository
	calls int
}

func (r *hourlyHistoryGateRepository) RecordXNodeTraffic(context.Context, string, string, uint64, string, wire.TrafficReport) error {
	r.calls++
	return errHourlyRecoveryPersist
}

func TestUnreadyHourlyHistoryRejectsBeforePersistingMixedReport(t *testing.T) {
	repo := &hourlyHistoryGateRepository{hourlySocketRepository: masterHourlyRepo()}
	repo.historyReady = false
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	peer := &agentPeer{nodeID: "node", agentID: masterHourlyAgent, bootID: masterHourlyBoot, trafficStreams: true, ctx: context.Background()}
	m.peers[peer.nodeID] = peer
	at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	report := wire.TrafficReport{ID: masterHourlyAgent + ":" + masterHourlyBoot + ":1", StreamID: masterHourlyBoot, Traffic: wire.Traffic{7: {19, 23}}}
	sample := wire.HourlySample{Owner: wire.HourlyOwner{ID: "owner", Generation: 4}, CounterGeneration: 1, EnrollmentID: "enrollment", UserID: 7, AccountingPeriod: "period", DayUTC: at.Format(time.DateOnly), From: at, Through: at.Add(time.Second), UploadBytes: 11, DownloadBytes: 13}
	for _, sampleOnly := range []bool{false, true} {
		mixed := report
		if sampleOnly {
			mixed.HourlySamples = []wire.HourlySample{sample}
		} else {
			mixed.DroppedHourlySamples = 1
		}
		if err := m.commitTraffic(context.Background(), peer, wire.New("traffic.report", mixed.ID, 0, mixed)); !errors.Is(err, ErrUnavailable) || repo.calls != 0 {
			t.Fatal("mixed report was persisted or acknowledged without history ingestion", err)
		}
	}
	if err := m.commitTraffic(context.Background(), peer, wire.New("traffic.report", report.ID, 0, report)); !errors.Is(err, errHourlyRecoveryPersist) || repo.calls != 1 {
		t.Fatal("history gate blocked ordinary native accounting", err)
	}
}
