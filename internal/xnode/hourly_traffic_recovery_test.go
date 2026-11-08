package xnode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

var errHourlyRecoveryPersist = errors.New("durable commit deliberately blocked")

type hourlyRecoveryTrafficRepository struct {
	Repository
	calls  int
	report wire.TrafficReport
}

// This fixture captures the complete report and deliberately refuses commit;
// it models implemented history ingestion without granting live admission.
func (*hourlyRecoveryTrafficRepository) HourlyTrafficHistoryReady() bool { return true }

func (r *hourlyRecoveryTrafficRepository) RecordXNodeTraffic(_ context.Context, node, agent string, sequence uint64, _ string, report wire.TrafficReport) error {
	if node != "node" || agent != masterHourlyAgent || sequence != 1 {
		return ErrInvalid
	}
	r.calls++
	r.report = report
	return errHourlyRecoveryPersist
}

func TestHourlyMixedJournalSurvivesLostAdmissionCapability(t *testing.T) {
	repo := &hourlyRecoveryTrafficRepository{}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	peer := &agentPeer{nodeID: "node", agentID: masterHourlyAgent, bootID: masterHourlyBoot, trafficStreams: true, ctx: context.Background()}
	m.peers[peer.nodeID] = peer
	at := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	report := wire.TrafficReport{ID: masterHourlyAgent + ":" + masterHourlyBoot + ":1", StreamID: masterHourlyBoot,
		Traffic: wire.Traffic{7: {19, 23}}, DroppedHourlySamples: 2,
		HourlySamples: []wire.HourlySample{{Owner: wire.HourlyOwner{ID: "owner", Generation: 4}, CounterGeneration: 1, EnrollmentID: "paid-enrollment", UserID: 7, AccountingPeriod: "paid-period", DayUTC: at.Format(time.DateOnly), From: at, Through: at.Add(time.Second), UploadBytes: 11, DownloadBytes: 13}}}
	message := wire.New("traffic.report", report.ID, 0, report)
	// The current connection has no hourly session after admission/storage
	// failure. Its authenticated old mixed journal must reach the transaction
	// unchanged. The persistence error must remain visible to journal retry.
	if err := m.commitTraffic(context.Background(), peer, message); !errors.Is(err, errHourlyRecoveryPersist) || repo.calls != 1 || !reflect.DeepEqual(repo.report, report) {
		t.Fatal("historical accounting depended on live admission or changed evidence", err, repo.calls)
	}
	if m.HourlyCapable(peer.nodeID) {
		t.Fatal("historical receipt support authorized live hourly access")
	}
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{}), at); !errors.Is(err, ErrInvalid) {
		t.Fatal("historical receipt opened the live hourly lane", err)
	}
	for _, invalid := range []wire.TrafficReport{
		{ID: report.ID, StreamID: "44444444444444444444444444444444", HourlySamples: report.HourlySamples},
		{ID: masterHourlyAgent + ":1", HourlySamples: report.HourlySamples},
		{ID: report.ID, StreamID: report.StreamID, HourlySamples: []wire.HourlySample{{UserID: 0}}},
	} {
		if err := m.commitTraffic(context.Background(), peer, wire.New("traffic.report", invalid.ID, 0, invalid)); !errors.Is(err, ErrInvalid) || repo.calls != 1 {
			t.Fatal("invalid historical sample/stream reached persistence", err)
		}
	}
	m.peers[peer.nodeID] = &agentPeer{}
	if err := m.commitTraffic(context.Background(), peer, message); !errors.Is(err, ErrUnavailable) || repo.calls != 1 {
		t.Fatal("replaced peer retained accounting authority", err)
	}
}

type hourlyReceiptRepository struct{ *hourlySocketRepository }

// Hide optional stream methods while retaining credit/history interfaces.
type hourlyWithoutStreamsRepository struct {
	Repository
	hourlyCreditRepository
	HourlyTrafficHistoryRepository
}

func TestHourlyHistoricalReceiptOfferRequiresBootStreamRepositoryAndOffer(t *testing.T) {
	for _, streams := range []bool{false, true} {
		for _, offer := range []string{"", "unknown", wire.TrafficStreamFormat} {
			base := masterHourlyRepo()
			var repo Repository = base
			if !streams {
				repo = hourlyWithoutStreamsRepository{base, base, base}
			}
			m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
			request := httptest.NewRequest(http.MethodGet, "/xnode", nil)
			request.Header.Set(wire.TrafficStreamHeader, offer)
			response := http.Header{}
			if session := m.hourlyOffer(request, response); session != "" {
				t.Fatal("historical receipt created a live admission offer")
			}
			want := streams && offer == wire.TrafficStreamFormat
			if (response.Get(wire.HourlyAccountingHeader) == wire.HourlyAccountingFormat) != want {
				t.Fatal("historical receipt compatibility mismatched stream support", streams, offer)
			}
			request.Header.Set(wire.HourlyHeader, wire.HourlyFormat)
			request.Header.Set(wire.HourlySessionHeader, masterHourlySession)
			response = http.Header{}
			if session := m.hourlyOffer(request, response); (session == masterHourlySession) != want || (response.Get(wire.HourlyHeader) == wire.HourlyFormat) != want {
				t.Fatal("live hourly offer omitted required boot-stream support", streams, offer)
			}
		}
	}
}

func TestHourlyTLSHelloCannotOmitTrafficStreamCapability(t *testing.T) {
	m, server, token := masterHourlyFixture(t, masterHourlyRepo())
	socket, response := masterHourlySocketWithStream(t, server, token, masterHourlySession, wire.HourlyFormat, true, false)
	defer socket.Close()
	if response.Header.Get(wire.HourlyHeader) != wire.HourlyFormat {
		t.Fatal("HTTP offer fixture did not reach provisional live negotiation")
	}
	peer := waitMasterHourlyPeer(t, m)
	if peer.trafficStreams || peer.hourlySession != "" || m.HourlyCapable(peer.nodeID) {
		t.Fatal("hello omitted the accounting stream but gained live hourly authority")
	}
}
