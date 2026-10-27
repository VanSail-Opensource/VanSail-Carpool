package xnode

// Traffic conflicts preserve ErrConflict matching while exposing only a fixed,
// credential-free diagnostic code to the socket logger.
type trafficConflict string

func (e trafficConflict) Error() string          { return string(e) }
func (e trafficConflict) Unwrap() error          { return ErrConflict }
func (e trafficConflict) frameErrorCode() string { return string(e) }

const (
	// ErrTrafficAgentMismatch rejects reports from a different enrolled agent.
	ErrTrafficAgentMismatch trafficConflict = "XNODE_TRAFFIC_AGENT_MISMATCH"
	// ErrTrafficDigestMismatch rejects a changed report at the committed cursor.
	ErrTrafficDigestMismatch trafficConflict = "XNODE_TRAFFIC_DIGEST_MISMATCH"
	// ErrTrafficSequenceGap requires missing journal entries before new traffic.
	ErrTrafficSequenceGap trafficConflict = "XNODE_TRAFFIC_SEQUENCE_GAP"
	// ErrTrafficSampleMissing rejects billable totals without period evidence.
	ErrTrafficSampleMissing trafficConflict = "XNODE_TRAFFIC_SAMPLE_MISSING"
	// ErrTrafficPeriodSettled protects immutable closed/review accounting.
	ErrTrafficPeriodSettled trafficConflict = "XNODE_TRAFFIC_PERIOD_SETTLED"
	// ErrTrafficOwnerMismatch rejects samples without period ownership evidence.
	ErrTrafficOwnerMismatch trafficConflict = "XNODE_TRAFFIC_OWNER_MISMATCH"
	// Missing journal references must never be skipped or acknowledged.
	ErrTrafficPeriodMissing     trafficConflict = "XNODE_TRAFFIC_PERIOD_MISSING"
	ErrTrafficPAYGPeriodMissing trafficConflict = "XNODE_TRAFFIC_PAYG_PERIOD_MISSING"
	ErrTrafficIdentityMissing   trafficConflict = "XNODE_TRAFFIC_IDENTITY_MISSING"
)

type trafficReportFailure struct {
	err      error
	sequence uint64
	samples  int
}

func (e trafficReportFailure) Error() string { return e.err.Error() }
func (e trafficReportFailure) Unwrap() error { return e.err }

type trafficSequenceGap struct{ received, expected uint64 }

func (e trafficSequenceGap) Error() string { return ErrTrafficSequenceGap.Error() }
func (e trafficSequenceGap) Unwrap() error { return ErrTrafficSequenceGap }

// NewTrafficSequenceGap retains only the two journal positions for diagnostics.
func NewTrafficSequenceGap(received, expected uint64) error {
	return trafficSequenceGap{received: received, expected: expected}
}

type trafficPeriodMissing struct{ period string }

func (e trafficPeriodMissing) Error() string { return ErrTrafficPeriodMissing.Error() }
func (e trafficPeriodMissing) Unwrap() error { return ErrTrafficPeriodMissing }

// NewTrafficPeriodMissing identifies a validated period reference, never a payload.
func NewTrafficPeriodMissing(period string) error { return trafficPeriodMissing{period: period} }
