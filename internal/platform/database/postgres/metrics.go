package postgres

import (
	"database/sql/driver"
	"sync/atomic"
	"time"
)

type transactionMetrics struct{ count, failed, nanos, retries atomic.Uint64 }

// Metrics is bounded in-memory telemetry; it contains no SQL or user labels.
type Metrics struct {
	Transactions, Failures, Retries uint64
	Seconds                         float64
}

func Snapshot(d driver.Driver) Metrics {
	value, ok := d.(compatDriver)
	if !ok || value.metrics == nil {
		return Metrics{}
	}
	m := value.metrics
	return Metrics{Transactions: m.count.Load(), Failures: m.failed.Load(), Retries: m.retries.Load(), Seconds: float64(m.nanos.Load()) / float64(time.Second)}
}

// ObserveRetry counts only safe whole-operation serialization/deadlock retries.
func ObserveRetry(d driver.Driver) {
	if value, ok := d.(compatDriver); ok && value.metrics != nil {
		value.metrics.retries.Add(1)
	}
}

type measuredTransaction struct {
	driver.Tx
	metrics  *transactionMetrics
	started  time.Time
	finished atomic.Bool
}

func (t *measuredTransaction) observe(err error) {
	if t.finished.Swap(true) {
		return
	}
	t.metrics.count.Add(1)
	t.metrics.nanos.Add(uint64(time.Since(t.started)))
	if err != nil {
		t.metrics.failed.Add(1)
	}
}
func (t *measuredTransaction) Commit() error   { err := t.Tx.Commit(); t.observe(err); return err }
func (t *measuredTransaction) Rollback() error { err := t.Tx.Rollback(); t.observe(err); return err }
