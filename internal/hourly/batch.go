package hourly

import "time"

// MaxBatchItems bounds one ordered receiver turn and its result allocation.
const MaxBatchItems = 128

// ObserveBatch applies an ordered batch using one clock and cleanup pass. Each
// Demand keeps its original ReceivedAt. Returned item errors correspond to input
// positions; an invalid item does not discard valid neighbors. Oversized input
// returns ErrInvalid without advancing time or changing state. Empty input is a
// no-op, matching an empty sequence of Observe calls.
func (a *Allocator) ObserveBatch(now time.Time, demands []Demand) ([]error, error) {
	if len(demands) > MaxBatchItems {
		return nil, ErrInvalid
	}
	if len(demands) == 0 {
		return nil, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	results := make([]error, len(demands))
	if !a.advance(now) {
		for i := range results {
			results[i] = ErrStale
		}
		return results, nil
	}
	for i, demand := range demands {
		results[i] = a.observe(now, demand)
	}
	return results, nil
}

// AcknowledgeBatch preserves input order and all exact-identity/native-drain
// checks of Acknowledge, but advances/prunes only once. Item failures do not
// discard other acknowledgments. Oversized input is rejected without mutation;
// empty input is a no-op.
func (a *Allocator) AcknowledgeBatch(now time.Time, acks []Ack) ([]error, error) {
	if len(acks) > MaxBatchItems {
		return nil, ErrInvalid
	}
	if len(acks) == 0 {
		return nil, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	results := make([]error, len(acks))
	if !a.advance(now) {
		for i := range results {
			results[i] = ErrStale
		}
		return results, nil
	}
	for i, ack := range acks {
		results[i] = a.acknowledge(now, ack)
	}
	return results, nil
}
