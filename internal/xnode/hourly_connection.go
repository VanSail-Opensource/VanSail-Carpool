package xnode

import (
	"net/http"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// HTTP headers precede hello. Echo only a supported valid offer provisionally;
// peer authority is installed only after hello repeats the capability.
func (m *Master) hourlyOffer(request *http.Request, response http.Header) string {
	if m.hourly == nil || !m.hourlyTrafficHistoryReady() {
		return ""
	}
	if _, supported := m.service.repo.(TrafficStreamRepository); !supported || request.Header.Get(wire.TrafficStreamHeader) != wire.TrafficStreamFormat {
		return ""
	}
	// Receipt compatibility is independent of live admission health. An agent
	// with a failed hourly store must still drain complete mixed journals.
	response.Set(wire.HourlyAccountingHeader, wire.HourlyAccountingFormat)
	if request.Header.Get(wire.HourlyHeader) != wire.HourlyFormat {
		return ""
	}
	session := request.Header.Get(wire.HourlySessionHeader)
	if !wire.ValidStreamID(session) {
		return ""
	}
	response.Set(wire.HourlyHeader, wire.HourlyFormat)
	response.Set(wire.HourlySessionHeader, session)
	return session
}

func (m *Master) connectHourlyPeer(peer *agentPeer) error {
	if peer.hourlySession == "" {
		return nil
	}
	m.mu.Lock()
	if m.peers[peer.nodeID] != peer || m.hourly == nil {
		m.mu.Unlock()
		return ErrUnavailable
	}
	co := m.hourly
	co.allocatorMu.Lock()
	err := co.allocator.Connect(m.service.now(), hourlyPeer(peer))
	co.allocatorMu.Unlock()
	m.mu.Unlock()
	if err == nil {
		m.requestHourlyCredit(peer, nil)
	}
	return err
}

func (m *Master) disconnectHourlyPeer(peer *agentPeer) {
	if peer.hourlySession == "" || m.hourly == nil {
		return
	}
	co := m.hourly
	co.allocatorMu.Lock()
	defer co.allocatorMu.Unlock()
	co.allocator.Disconnect(m.service.now(), hourlyPeer(peer))
}
