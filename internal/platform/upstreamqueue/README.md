# Upstream queue
- `queue_part2.go` continues the focused implementation from its original package module.

This package owns the in-memory admission queues for synchronous provider API
calls. Each provider gets a separate bounded FIFO and context-owned workers,
so slow X-Node traffic cannot block Emby traffic (or vice versa).

`Do` and `Execute` wait for capacity, provider pacing, and the final result. The
caller's context cancels all three phases. Application shutdown also cancels the
active call and stops accepting new work without closing a channel that may
still have concurrent senders.

`Config.Workers` defaults to one. A single scheduler preserves the provider-wide
minimum start interval while admitting at most the configured number of calls.
Shutdown cancels and joins all active workers. Provider adapters select bounded
concurrency and account keys; IP Lookup retains its explicitly serialized lanes.
`concurrency_test.go` covers bounded overlap, shared pacing and shutdown in hosted CI.

This queue is intentionally not durable. Database outbox jobs remain the source
of truth for retryable mutations; an outbox handler enters this queue immediately
before invoking its provider adapter.

- `queue.go` owns bounded FIFO admission, provider pacing, lifecycle transitions, shutdown, and cancellation merging.
- `result.go` exposes typed `Do` and error-only `Execute` helpers with panic containment and caller cancellation.
- `queue_test.go` verifies validation, results, panic recovery, backpressure, pacing, active-call cancellation, and shutdown state.
- `queue_part2.go` contains queue worker and lifecycle helpers.
- `config.go` exposes read-only queue configuration accessors.

`ordering.go` exposes WithKey for per-resource FIFO. The existing scheduler bounds pending work and active workers, skips occupied keys, and retains one provider-wide pacing clock. Cancellation and shutdown join active calls.

`ordering_test.go` checks that an occupied account does not stall another account.

The scheduler alone owns the `Capacity` pending slots. Its unbuffered intake
prevents a second hidden backlog; callers waiting for admission retain context
cancellation. `queue_test.go` waits for admission explicitly and checks that a
full backlog rejects another admission at the caller's deadline.
