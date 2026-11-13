package xnode

import (
	"context"
)

type nodeEnrollmentLock struct {
	held chan struct{}
	refs int
}

// Enrollment, registration and deletion serialize only the same node. Database
// waits never hold Master.mu, which also serves telemetry and command delivery.
// References include waiters, and the last waiter/holder removes the entry.
func (m *Master) lockEnrollment(ctx context.Context, node string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.enrollmentMu.Lock()
	if m.enrollmentLocks == nil {
		m.enrollmentLocks = map[string]*nodeEnrollmentLock{}
	}
	gate := m.enrollmentLocks[node]
	if gate == nil {
		gate = &nodeEnrollmentLock{held: make(chan struct{}, 1)}
		m.enrollmentLocks[node] = gate
	}
	gate.refs++
	m.enrollmentMu.Unlock()
	drop := func() {
		m.enrollmentMu.Lock()
		gate.refs--
		if gate.refs == 0 {
			delete(m.enrollmentLocks, node)
		}
		m.enrollmentMu.Unlock()
	}
	select {
	case gate.held <- struct{}{}:
		release := func() { <-gate.held; drop() }
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		return release, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}
