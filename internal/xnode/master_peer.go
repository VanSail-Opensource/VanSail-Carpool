package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type agentPeer struct {
	trafficRetry          trafficRetry // mu; bounded retries for an unchanged rejected journal head
	paygRecoveryAt        time.Time    // mu; bounded recovery requeue after interrupted attempts
	paygReady             bool         // mu; true only after a committed journal barrier
	paygRecoveryQueued    bool         // mu; durable recovery is coalesced per boot
	paygOverlayKey        string       // syncMu; disposable authorization projection
	publicIP              string
	compactTelemetry      bool // immutable, negotiated upgrade header plus hello capability
	ddnsChanges           bool // immutable, independently negotiated DDNS changes
	protectedMu           sync.Mutex
	protectedDigest       string
	protectedIPs          []string // mu; last acknowledged protected bank
	protectedAt           time.Time
	nodeID                string
	agentID               string
	bootID                string
	socket                *websocket.Conn
	queue                 *upstreamqueue.Queue
	ctx                   context.Context
	cancel                context.CancelFunc
	mu                    sync.Mutex
	syncMu                sync.Mutex
	revision              uint64
	digest                string
	pending               map[string]chan wire.Message
	lastControls          map[int]bool
	compiled              *compiledState  // protected by syncMu
	compileFailure        *compileFailure // protected by syncMu
	wireState             *preparedWire   // protected by syncMu
	forceSync             bool            // protected by mu; coalesced by the existing reconciler
	failedDigest          string          // protected by mu
	retryAt               time.Time       // protected by mu
	retryDelay            time.Duration   // protected by mu
	pong                  chan string
	firewallSyncMu        sync.Mutex
	firewallSourceMu      sync.Mutex
	firewallJoined        bool
	geoIPTransferMu       sync.Mutex
	geoIPStateMu          sync.Mutex
	geoIPGeneration       string    // protected by geoIPStateMu; staged country generation on this peer
	geoIPQueuedGeneration string    // protected by geoIPStateMu
	geoIPQueuedAt         time.Time // protected by geoIPStateMu; throttles duplicate outbox intents
}
