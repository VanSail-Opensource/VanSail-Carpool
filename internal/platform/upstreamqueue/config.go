package upstreamqueue

import "time"

// Name returns the provider label used in diagnostic errors.

func (q *Queue) Name() string {
	if q == nil {
		return ""
	}
	return q.name
}

// Capacity returns the maximum number of calls waiting behind the active call.

func (q *Queue) Capacity() int {
	if q == nil {
		return 0
	}
	return q.capacity
}

// MinInterval returns the configured minimum delay between call start times.

func (q *Queue) MinInterval() time.Duration {
	if q == nil {
		return 0
	}
	return q.minInterval
}
