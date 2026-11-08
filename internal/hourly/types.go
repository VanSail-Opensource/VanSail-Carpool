// Package hourly allocates memory-only account rate and burst grants.
package hourly

import "time"

const (
	DecisionInterval        = 2 * time.Second
	MaxLifetime             = 10 * time.Second
	MaxBurstBytes    uint32 = 16 << 10
)

// Owner is an immutable account generation; ID is the panel's opaque user ID.
type Owner struct {
	ID         string
	Generation uint64
}

// Peer binds grants to one authenticated node process and control session.
type Peer struct{ NodeID, AgentID, BootID, SessionID string }

// Demand.ReceivedAt is captured once by the ordered authenticated receiver.
// Lifetime is measured from the original agent demand start, not master issue.
type Demand struct {
	Owner          Owner
	Peer           Peer
	Nonce          string
	ReceivedAt     time.Time
	Lifetime       time.Duration
	PendingBytes   uint64
	MinAtomicBytes uint32
}

// Policy is supplied by authoritative commerce; an absent policy grants nothing.
type Policy struct {
	Owner              Owner
	PaidThrough        time.Time
	RateBytesPerSecond uint64
	BurstBytes         uint32
}

// Grant is a transport action. ConservativeUntil is the master reservation upper
// bound, not an agent-side wall-clock expiry or a new lifetime origin.
type Grant struct {
	Owner              Owner
	Peer               Peer
	Nonce              string
	Sequence           uint64
	DayUTC             string
	Lifetime           time.Duration
	RateBytesPerSecond uint64
	BurstBytes         uint32
	ConservativeUntil  time.Time
}

// Ack is accepted only for the exact current peer and issued grant identity.
type Ack struct {
	Owner    Owner
	Peer     Peer
	Nonce    string
	Sequence uint64
	Applied  bool
}

// Limits bound process-local state; zero values select documented defaults.
type Limits struct {
	MaxAccounts, MaxPeers, MaxDemandsPerAccount, MaxNoncesPerAccount int
}

// Reservation projects the largest potentially live grant for one peer/owner.
type Reservation struct {
	Owner              Owner
	Peer               Peer
	RateBytesPerSecond uint64
	BurstBytes         uint32
	Until              time.Time
}
