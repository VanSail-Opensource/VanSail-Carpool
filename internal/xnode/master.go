package xnode

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

type Master struct {
	service                  *Service
	logger                   *slog.Logger
	ctx                      context.Context
	cancel                   context.CancelFunc
	mu                       sync.RWMutex
	peers                    map[string]*agentPeer
	live                     map[string]liveSnapshot
	globalBlocks             map[int]bool
	stateWake                chan struct{}
	wg                       sync.WaitGroup
	operations               int
	firewallInventory        FirewallInventory
	firewallWake             chan struct{}
	firewallSourcesRunning   int
	firewallInventoryVersion uint64
}
type agentPeer struct {
	nodeID           string
	agentID          string
	bootID           string
	socket           *websocket.Conn
	queue            *upstreamqueue.Queue
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.Mutex
	syncMu           sync.Mutex
	revision         uint64
	digest           string
	pending          map[string]chan wire.Message
	lastControls     map[int]bool
	compiled         *compiledState  // protected by syncMu
	compileFailure   *compileFailure // protected by syncMu
	wireState        *preparedWire   // protected by syncMu
	forceSync        bool            // protected by mu; coalesced by the existing reconciler
	failedDigest     string          // protected by mu
	retryAt          time.Time       // protected by mu
	retryDelay       time.Duration   // protected by mu
	pong             chan string
	firewallSyncMu   sync.Mutex
	firewallSourceMu sync.Mutex
	countryMu        sync.Mutex
	geoIPGeneration  string
	firewallJoined   bool
}

func NewMaster(service *Service, logger *slog.Logger) *Master {
	m := &Master{service: service, logger: logger, peers: map[string]*agentPeer{}, live: map[string]liveSnapshot{}, globalBlocks: map[int]bool{}}
	service.master = m
	m.stateWake = make(chan struct{}, 1)
	m.firewallWake = make(chan struct{}, 1)
	return m
}
func (m *Master) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx != nil {
		return ErrConflict
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.wg.Add(1)
	go m.run()
	if _, ok := m.service.repo.(firewallRepository); ok {
		m.wg.Add(1)
		go m.runFirewall()
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.service.RunAccounting(m.ctx, func(error) {
			m.logger.Warn("accounting period remains pending", "error_code", "XNODE_ACCOUNTING_PENDING")
		})
	}()
	if m.service.legacySync != nil {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			tick := time.NewTicker(15 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-m.ctx.Done():
					return
				case <-tick.C:
					ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
					err := m.service.legacySync(ctx)
					cancel()
					if err != nil && m.ctx.Err() == nil {
						m.logger.Warn("legacy traffic sync remains pending", "error_code", "XNODE_LEGACY_SYNC_PENDING")
					}
				}
			}
		}()
	}
	return nil
}

// Disconnect forces a new authenticated handshake after an enrollment credential changes.
func (m *Master) Disconnect(nodeID string) {
	m.mu.RLock()
	peer := m.peers[nodeID]
	m.mu.RUnlock()
	if peer != nil {
		peer.cancel()
		_ = peer.socket.Close()
	}
}
func (m *Master) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	peers := make([]*agentPeer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	m.mu.Unlock()
	for _, peer := range peers {
		peer.cancel()
		if err := peer.socket.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			m.logger.Debug("close agent socket", "node_id", peer.nodeID)
		}
	}
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (m *Master) run() {
	defer m.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-tick.C:
		case <-m.stateWake:
		}
		ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
		if err := m.reconcile(ctx); err != nil && !errors.Is(err, context.Canceled) {
			m.logger.Warn("reconcile X-Node state", "error", err)
		}
		cancel()
	}
}

// requestSync never compiles or waits on external work in the socket reader.
// It wakes the existing reconciler rather than adding another execution queue.
func (m *Master) requestSync(peer *agentPeer) {
	peer.mu.Lock()
	peer.forceSync = true
	peer.mu.Unlock()
	select {
	case m.stateWake <- struct{}{}:
	default:
	}
}

// Wait joins database-dependent controller tasks after Shutdown cancels them.
func (m *Master) Wait() { m.wg.Wait() }
func (peer *agentPeer) send(ctx context.Context, message wire.Message) error {
	return upstreamqueue.Execute(ctx, peer.queue, func(callCtx context.Context) error {
		if callCtx.Err() != nil {
			return callCtx.Err()
		}
		deadline := time.Now().Add(10 * time.Second)
		if value, ok := callCtx.Deadline(); ok && value.Before(deadline) {
			deadline = value
		}
		if err := peer.socket.SetWriteDeadline(deadline); err != nil {
			return err
		}
		return peer.socket.WriteJSON(message)
	})
}
