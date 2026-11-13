package xnode

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Master struct {
	purgedActivity         map[int]*purgeActivityFence // mu; bounded volatile presentation fences
	purgeActivityScopes    int
	purgeActivitySession   uint64
	purgeActivitySweepAt   time.Time
	hourly                 *hourlyCoordinator
	hourlyCredits          *hourlyCreditCoordinator
	automaticLeases        *automaticLeaseManager
	automaticFreshness     atomic.Int64
	automaticWake          chan struct{}
	syncCursor             int
	compute                *computePool
	trafficMetrics         trafficObservations
	trafficBatcher         *trafficBatcher
	trafficInboundBytes    atomic.Int64
	protectedReleaseDigest string
	managedNodeIPs         map[string][]string
	managedHostIPs         []string
	ddnsProtectedIPs       []string
	ddnsNodeIPs            map[string][]string
	ddnsProtectionMu       sync.Mutex
	ddnsProtectionDigest   string
	ddnsProtectionVersion  int64
	hostOrigins            map[string]hostOrigin
	whitelistStates        map[string]whitelistNodeState
	service                *Service
	logger                 *slog.Logger
	ctx                    context.Context
	cancel                 context.CancelFunc
	mu                     sync.RWMutex
	enrollmentMu           sync.Mutex
	enrollmentLocks        map[string]*nodeEnrollmentLock
	peers                  map[string]*agentPeer
	live                   map[string]liveSnapshot
	globalBlocks           map[int]bool
	stateWake              chan struct{}
	wg                     sync.WaitGroup
	operations             int
	memberOperations       int
	firewallWake           chan struct{}
	firewallSourcesRunning int
}

func NewMaster(service *Service, logger *slog.Logger) *Master {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	m := &Master{service: service, logger: logger, peers: map[string]*agentPeer{}, live: map[string]liveSnapshot{}, globalBlocks: map[int]bool{}, automaticLeases: newAutomaticLeaseManager(rand.Text())}
	m.compute = newComputePool(m)
	if m.hourlyCreditSupported() {
		m.hourly = newHourlyCoordinator(service.now())
		m.hourlyCredits = newHourlyCreditCoordinator()
	}
	service.master = m
	m.stateWake = make(chan struct{}, 1)
	m.firewallWake = make(chan struct{}, 1)
	m.automaticWake = make(chan struct{}, 1)
	m.automaticFreshness.Store(int64(reportWindow(2)))
	return m
}
func (m *Master) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx != nil {
		return ErrConflict
	}
	if m.service.settings != nil {
		global, err := m.service.Global(ctx)
		if err != nil {
			return err
		}
		m.configureAutomaticLeases(global)
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	if m.hourly != nil {
		m.hourly = newHourlyCoordinator(m.service.now())
		m.wg.Add(2)
		go m.runHourly()
		go m.runHourlyCredits(m.ctx)
	}
	m.wg.Add(1)
	go m.runAutomaticLeases()
	if repo, ok := m.service.repo.(TrafficBatchRepository); ok {
		m.trafficBatcher = &trafficBatcher{repo: repo, input: make(chan trafficBatchRequest, MaxTrafficBatchReports)}
		m.wg.Add(1)
		go func() { defer m.wg.Done(); m.trafficBatcher.run(m.ctx) }()
	}
	m.wg.Add(1)
	go m.compute.run(m.ctx)
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
	return nil
}

// Disconnect forces a new authenticated handshake after an enrollment credential changes.
func (m *Master) Disconnect(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectPeerLocked(nodeID)
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
