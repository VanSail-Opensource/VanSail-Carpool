package xnode

// telemetryFrameFailure retains only a fixed validation stage. Agent payloads,
// identities, IPs and decoder errors must never enter rejection diagnostics.
type telemetryFrameFailure struct {
	reason string
	cause  error
}

func (e telemetryFrameFailure) Error() string          { return "XNODE_TELEMETRY_REJECTED" }
func (e telemetryFrameFailure) Unwrap() error          { return e.cause }
func (e telemetryFrameFailure) frameErrorCode() string { return e.Error() }

func rejectTelemetry(reason string, cause error) error {
	return telemetryFrameFailure{reason: reason, cause: cause}
}
