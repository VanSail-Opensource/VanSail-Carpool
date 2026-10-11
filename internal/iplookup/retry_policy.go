package iplookup

import (
	"errors"
	"time"
)

// RetryOutboxKind drives background repair of partial reputation reports.
const ReportRetryOutboxKind = "ip_lookup_report_retry"

// LiveRetryOutboxKind drives background repair of partial live snapshots.
const LiveRetryOutboxKind = "ip_lookup_live_retry"

// RetryPolicy bounds background repair of partially failed lookups. A nil
// policy on a stored config means the defaults below apply, so existing
// installations keep repairing without an explicit settings round-trip.
type RetryPolicy struct {
	Enabled      bool `json:"enabled"`
	MaxAttempts  int  `json:"maxAttempts"`
	DelaySeconds int  `json:"delaySeconds"`
}

// DefaultRetryPolicy repairs up to three times, waiting a minute before the
// first attempt and between attempts.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Enabled: true, MaxAttempts: 3, DelaySeconds: 60}
}

// resolveRetryPolicy normalizes stored values: enabled policies fall back to
// the defaults for omitted numbers, disabled policies keep explicit values.
func resolveRetryPolicy(p *RetryPolicy) RetryPolicy {
	if p == nil {
		return DefaultRetryPolicy()
	}
	resolved := *p
	if resolved.Enabled {
		if resolved.MaxAttempts <= 0 {
			resolved.MaxAttempts = 3
		}
		if resolved.DelaySeconds <= 0 {
			resolved.DelaySeconds = 60
		}
	}
	return resolved
}

// RetryPolicyOf resolves the effective policy for a decoded config.
func RetryPolicyOf(c Config) RetryPolicy {
	return resolveRetryPolicy(c.Retry)
}

// NextRetryDelay reports how long to wait before the next repair attempt.
// A false delay means background repair is disabled.
func NextRetryDelay(c Config) (time.Duration, bool) {
	policy := resolveRetryPolicy(c.Retry)
	if !policy.Enabled {
		return 0, false
	}
	return time.Duration(policy.DelaySeconds) * time.Second, true
}

// RetryAllowed reports whether one more repair attempt fits the policy.
func RetryAllowed(c Config, attempt int) bool {
	policy := resolveRetryPolicy(c.Retry)
	return policy.Enabled && attempt >= 1 && attempt <= policy.MaxAttempts
}

// ErrRepairConflict signals that the cached row being repaired changed shape
// concurrently, so the repair chain should stop without an infra retry.
var ErrRepairConflict = errors.New("ip lookup repair target changed")

// ErrRepairGone signals that the cached row being repaired no longer exists.
var ErrRepairGone = errors.New("ip lookup repair target is gone")

// ReportRetryPayload addresses one background repair attempt of a partial
// reputation report. The attempt counter makes each scheduled job distinct so
// the outbox payload deduplication cannot collapse a repair chain.
type ReportRetryPayload struct {
	ReportID string `json:"reportId"`
	Attempt  int    `json:"attempt"`
}

// LiveRetryPayload addresses one background repair attempt of a partial live
// snapshot's failed sections.
type LiveRetryPayload struct {
	SnapshotID string `json:"snapshotId"`
	Attempt    int    `json:"attempt"`
}

// FailedStages lists checkpoint stages that qualify for background repair:
// transport failures and stages interrupted mid-run. Refusal-skipped and
// disabled stages deliberately stay excluded.
func FailedStages(r Report) []string {
	if r.Checkpoint == nil {
		return nil
	}
	var failed []string
	for _, stage := range r.Checkpoint.Stages {
		if stage.Status == "error" || stage.Status == "processing" || stage.Status == "queued" {
			failed = append(failed, stage.ID)
		}
	}
	if stage := r.Checkpoint.RPKI; stage != nil && (stage.Status == "error" || stage.Status == "processing" || stage.Status == "queued") {
		failed = append(failed, "rpki")
	}
	return failed
}

// PrepareRepair resets failed stages for a fresh pass and recomputes the
// aggregate completeness flag exactly from the frozen per-stage markers.
func PrepareRepair(r *Report) {
	if r.Checkpoint == nil {
		return
	}
	complete := true
	for i := range r.Checkpoint.Stages {
		stage := &r.Checkpoint.Stages[i]
		switch stage.Status {
		case "partial":
			complete = false
		case "success":
			complete = complete && stage.Complete
		case "error", "processing":
			stage.Status, stage.Attempted, stage.Complete = "queued", false, false
		}
	}
	r.Checkpoint.Complete = complete
	if stage := r.Checkpoint.RPKI; stage != nil && (stage.Status == "error" || stage.Status == "processing") {
		stage.Status, stage.Attempted = "queued", false
	}
}
