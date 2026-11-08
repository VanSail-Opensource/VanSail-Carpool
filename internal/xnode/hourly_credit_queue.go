package xnode

import (
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const maxPendingHourlyCreditAcks = 1024
const maxHourlyCreditWorkers = 4
const maxHourlyCreditPeerStates = 2048
const maxHourlyCreditDemandHints = 8192

type hourlyCreditHint struct {
	demand   wire.HourlyDemand
	seenAt   time.Time
	activeAt time.Time
}

type hourlyCreditObserved struct {
	bytes int64
	at    time.Time
}

type hourlyCreditRate struct {
	bytesPerSecond float64 // disposable sizing hint; never financial accounting
	at             time.Time
}

type hourlyCreditPeerState struct {
	pending      map[string]wire.HourlyCreditAck
	running      bool
	retryAt      time.Time
	hints        map[wire.HourlyOwner]hourlyCreditHint
	delivered    map[string]int64
	uncertain    map[string]bool
	intents      map[wire.HourlyOwner]model.HourlyCreditRequest
	retire       map[wire.HourlyOwner]bool
	retireAll    bool
	work         bool
	createdAt    time.Time
	scanIssue    int64
	scanned      bool
	ownerCursor  uint64
	intentCursor uint64
	observed     map[string]hourlyCreditObserved
	rates        map[wire.HourlyOwner]hourlyCreditRate
	roundTrip    time.Duration
}

// This is disposable coalesced receipt state. Durable accepted proofs live in
// the existing ledger and replay from the agent until acknowledged. Every wire
// operation still uses the established per-node queue and command executor.
type hourlyCreditCoordinator struct {
	mu           sync.Mutex
	peers        map[*agentPeer]*hourlyCreditPeerState
	wake         chan struct{}
	cursor       uint64
	workers      int
	hintCount    int
	activeAgents map[string]*agentPeer
}

func newHourlyCreditCoordinator() *hourlyCreditCoordinator {
	return &hourlyCreditCoordinator{peers: map[*agentPeer]*hourlyCreditPeerState{}, wake: make(chan struct{}, 1), activeAgents: map[string]*agentPeer{}}
}

func (c *hourlyCreditCoordinator) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (m *Master) queueHourlyCreditAck(peer *agentPeer, ack wire.HourlyCreditAck) error {
	if !m.currentHourlyCreditPeer(peer) || m.hourlyCredits == nil {
		return ErrUnavailable
	}
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.peers[peer]
	if state == nil {
		state = c.stateLocked(peer, m.service.now())
		if state == nil {
			return ErrUnavailable
		}
	}
	if previous, exists := state.pending[ack.ReservationID]; exists {
		if previous != ack {
			return ErrConflict
		}
		return nil
	}
	if len(state.pending) >= maxPendingHourlyCreditAcks {
		return ErrUnavailable
	}
	state.pending[ack.ReservationID] = ack
	c.signal()
	return nil
}

func (c *hourlyCreditCoordinator) stateLocked(peer *agentPeer, at time.Time) *hourlyCreditPeerState {
	if state := c.peers[peer]; state != nil {
		return state
	}
	if len(c.peers) >= maxHourlyCreditPeerStates {
		return nil
	}
	state := &hourlyCreditPeerState{pending: map[string]wire.HourlyCreditAck{}, hints: map[wire.HourlyOwner]hourlyCreditHint{}, delivered: map[string]int64{}, uncertain: map[string]bool{}, intents: map[wire.HourlyOwner]model.HourlyCreditRequest{}, retire: map[wire.HourlyOwner]bool{}, createdAt: at}
	state.observed = map[string]hourlyCreditObserved{}
	state.rates = map[wire.HourlyOwner]hourlyCreditRate{}
	c.peers[peer] = state
	return state
}
