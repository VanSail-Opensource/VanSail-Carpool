package xnode

import (
	"net/netip"
	"sync"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const maximumAutomaticLeases = 100000
const maximumLeaseSources = 8
const automaticLeaseBatch = 256

type AutomaticWhitelistConfig struct {
	Enabled       bool `json:"enabled"`
	DurationHours int  `json:"durationHours"`
}

func defaultAutomaticWhitelist() *AutomaticWhitelistConfig {
	return &AutomaticWhitelistConfig{Enabled: true, DurationHours: 24}
}

type AutomaticLeaseGrant = wire.AutomaticLeaseGrant
type AutomaticLeaseObservation struct {
	IP, Node string
	At       time.Time
}
type automaticLeaseSource struct {
	node string
	last time.Time
}
type automaticLease struct {
	owners                      map[int64]time.Time
	ip                          netip.Addr
	first, last, started, until time.Time
	renewed                     *time.Time
	generation, observations    uint64
	renewals                    uint64
	sources                     [maximumLeaseSources]automaticLeaseSource
	sourceCount, index          int
	sourcesTruncated            bool
}
type automaticBanRelease struct{ grant AutomaticLeaseGrant }

type automaticLeaseManager struct {
	mu                                  sync.Mutex
	ownerAssociations                   int
	associationsTruncated               bool
	epoch                               string
	config                              AutomaticWhitelistConfig
	leases                              map[netip.Addr]*automaticLease
	expiry                              automaticLeaseHeap
	pendingBans                         map[netip.Addr]*automaticBanRelease
	generation, revision, snapshotAt    uint64
	snapshot                            []AutomaticLeaseGrant
	created, renewed, expired, rejected uint64
}

func newAutomaticLeaseManager(epoch string) *automaticLeaseManager {
	return &automaticLeaseManager{epoch: epoch, config: *defaultAutomaticWhitelist(), leases: map[netip.Addr]*automaticLease{}, pendingBans: map[netip.Addr]*automaticBanRelease{}}
}

func (m *automaticLeaseManager) configure(config AutomaticWhitelistConfig) {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()
}
