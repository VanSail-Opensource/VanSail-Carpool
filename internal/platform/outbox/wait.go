package outbox

import (
	"fmt"
	"time"
)

// WaitError keeps the same durable claim while an external prerequisite is
// unresolved. Dependency waits do not consume delivery attempts.
type WaitError struct {
	Reason string
	Cause  error
	After  time.Duration
}

func (e *WaitError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Reason, e.Cause)
	}
	return e.Reason
}

func (e *WaitError) Unwrap() error             { return e.Cause }
func (e *WaitError) Is(target error) bool      { return target == ErrBusy }
func (e *WaitError) RetryDelay() time.Duration { return e.After }

// ReviewError records a terminal attempt requiring new operator evidence.
type ReviewError struct {
	Reason string
	Cause  error
}

func (e *ReviewError) Error() string { return "review_required: " + e.Reason }
func (e *ReviewError) Unwrap() error { return e.Cause }

// TerminalError identifies a known failed outcome that retrying cannot repair.
type TerminalError struct {
	Reason string
	Cause  error
}

func (e *TerminalError) Error() string { return fmt.Sprintf("terminal: %s: %v", e.Reason, e.Cause) }
func (e *TerminalError) Unwrap() error { return e.Cause }
