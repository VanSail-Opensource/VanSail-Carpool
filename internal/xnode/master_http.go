package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func (m *Master) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	lifetime := m.ctx
	m.mu.RUnlock()
	if lifetime == nil || lifetime.Err() != nil {
		http.Error(w, "agent controller unavailable", http.StatusServiceUnavailable)
		return
	}
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(raw) != 43 {
		http.Error(w, "agent authentication required", http.StatusUnauthorized)
		return
	}
	hash := sha256.Sum256([]byte(raw))
	node, err := m.service.repo.XNodeNodeByToken(r.Context(), hash[:])
	if err != nil {
		http.Error(w, "agent authentication required", http.StatusUnauthorized)
		return
	}
	protocolOK := false
	for _, protocol := range websocket.Subprotocols(r) {
		if protocol == "x-node.v1" {
			protocolOK = true
		}
	}
	if !websocket.IsWebSocketUpgrade(r) || !protocolOK {
		w.Header().Set("Sec-WebSocket-Protocol", "x-node.v1")
		http.Error(w, "agent WebSocket upgrade required", http.StatusUpgradeRequired)
		return
	}
	compactOffer := r.Header.Get(wire.TelemetryFormatHeader) == wire.CompactTelemetryFormat
	openConnectionsOffer := r.Header.Get(wire.OpenConnectionsHeader) == wire.OpenConnectionsFormat
	activityOffer := r.Header.Get(wire.ActivityObservationsHeader) == wire.ActivityObservationsFormat
	ddnsOffer := r.Header.Get(wire.DDNSChangesHeader) == wire.DDNSChangesFormat
	reportOffer := r.Header.Get(wire.ReportFramingHeader) == wire.ZstdReportFraming
	_, streamSupported := m.service.repo.(TrafficStreamRepository)
	streamOffer := streamSupported && r.Header.Get(wire.TrafficStreamHeader) == wire.TrafficStreamFormat
	responseHeaders := http.Header{}
	hourlyOffer := m.hourlyOffer(r, responseHeaders)
	if activityOffer {
		responseHeaders.Set(wire.ActivityObservationsHeader, wire.ActivityObservationsFormat)
	}
	if streamOffer {
		responseHeaders.Set(wire.TrafficStreamHeader, wire.TrafficStreamFormat)
	}
	if reportOffer {
		responseHeaders.Set(wire.ReportFramingHeader, wire.ZstdReportFraming)
	}
	if openConnectionsOffer {
		responseHeaders.Set(wire.OpenConnectionsHeader, wire.OpenConnectionsFormat)
	}
	if compactOffer {
		responseHeaders.Set(wire.TelemetryFormatHeader, wire.CompactTelemetryFormat)
	}
	if ddnsOffer {
		responseHeaders.Set(wire.DDNSChangesHeader, wire.DDNSChangesFormat)
	}
	if r.Header.Get(wire.ComputePoolHeader) == wire.ComputePoolFormat {
		responseHeaders.Set(wire.ComputePoolHeader, wire.ComputePoolFormat)
	}
	upgrader := websocket.Upgrader{EnableCompression: true, Subprotocols: []string{"x-node.v1"}, CheckOrigin: func(*http.Request) bool { return true }, HandshakeTimeout: 15 * time.Second}
	socket, err := upgrader.Upgrade(w, r, responseHeaders)
	if err != nil {
		return
	}
	defer socket.Close()
	// Master commands/acks stay uncompressed; only the agent report writer opts
	// into compression. This remains inside the single-writer contract.
	socket.EnableWriteCompression(false)
	socket.SetReadLimit(wire.MaxReportFrameBytes)
	if err = socket.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return
	}
	message, err := readAgentFrame(socket)
	if err != nil || message.Type != "agent.hello" {
		return
	}
	var hello wire.AgentHello
	if err = wire.Decode(message.Data, &hello); err != nil || len(hello.AgentID) != 32 || len(hello.BootID) != 32 {
		return
	}
	var reportDecoder *wire.ReportDecoder
	if reportOffer && containsCapability(hello.Capabilities, wire.ReportFramingCapability) {
		reportDecoder, err = wire.NewReportDecoder()
		if err != nil {
			return
		}
		defer reportDecoder.Close()
	}
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "xnode-" + node.ID, Capacity: 128})
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(lifetime)
	defer cancel()
	if err = queue.Start(ctx); err != nil {
		return
	}
	defer queue.Shutdown(context.Background())
	peer := &agentPeer{logger: m.logger, nodeID: node.ID, agentID: hello.AgentID, bootID: hello.BootID, socket: socket, queue: queue, ctx: ctx, cancel: cancel, revision: message.Revision, pending: map[string]chan wire.Message{}, lastControls: map[int]bool{}}
	if hourlyOffer != "" && containsCapability(hello.Capabilities, wire.HourlyCapability) && containsCapability(hello.Capabilities, wire.TrafficStreamCapability) && wire.ValidStreamID(hello.AgentID) && wire.ValidStreamID(hello.BootID) {
		peer.hourlySession = hourlyOffer
		peer.hourlySender = newHourlyGrantSender()
	}
	peer.computePool = r.Header.Get(wire.ComputePoolHeader) == wire.ComputePoolFormat && containsCapability(hello.Capabilities, wire.ComputePoolCapability)
	peer.compactTelemetry = compactOffer && containsCapability(hello.Capabilities, wire.CompactTelemetryCapability)
	peer.openConnections = openConnectionsOffer && containsCapability(hello.Capabilities, wire.OpenConnectionsCapability)
	peer.activityReports = activityOffer && containsCapability(hello.Capabilities, wire.ActivityObservationsCapability)
	peer.ddnsChanges = ddnsOffer && containsCapability(hello.Capabilities, wire.DDNSChangesCapability)
	peer.reportDecoder = reportDecoder
	peer.trafficStreams = streamOffer && containsCapability(hello.Capabilities, wire.TrafficStreamCapability) && wire.ValidStreamID(hello.BootID)
	if err = m.refreshDDNSProtection(ctx); err != nil {
		return
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && r.Header.Get("X-Forwarded-For") == "" {
		peer.publicIP, _ = FirewallPublicIP(host)
	}
	if err = m.registerPeer(ctx, node.ID, hash[:], hello, peer); err != nil {
		return
	}
	m.rememberNodeIPs(peer, json.RawMessage(`{}`))
	defer func() {
		m.disconnectHourlyPeer(peer)
		m.mu.Lock()
		if m.peers[node.ID] == peer {
			delete(m.peers, node.ID)
			snapshot := m.live[node.ID]
			snapshot.receivedAt = time.Time{}
			m.live[node.ID] = snapshot
		}
		m.mu.Unlock()
		m.wg.Done()
	}()
	if err = m.connectHourlyPeer(peer); err != nil {
		return
	}
	if err = socket.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
		return
	}
	socket.SetPongHandler(func(payload string) error {
		peer.firewallPong(payload)
		return socket.SetReadDeadline(time.Now().Add(90 * time.Second))
	})
	socket.SetPingHandler(func(payload string) error {
		return upstreamqueue.Execute(ctx, queue, func(context.Context) error {
			return socket.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(10*time.Second))
		})
	})
	receivers := m.startPeerReceivers(peer)
	defer receivers.stop()
	for {
		message, err = readAgentFrame(socket, peer.reportDecoder)
		if err != nil {
			return
		}
		if err = socket.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return
		}
		if err = receivers.offer(message); err != nil {
			return
		}
	}
}
