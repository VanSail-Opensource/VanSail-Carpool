package iplookup

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRetryPolicyResolutionKeepsDefaultsAndExplicitValues(t *testing.T) {
	t.Parallel()
	defaults := RetryPolicyOf(Config{})
	if defaults != DefaultRetryPolicy() {
		t.Fatalf("nil policy lost defaults: %+v", defaults)
	}
	resolved := RetryPolicyOf(Config{Retry: &RetryPolicy{Enabled: true}})
	if resolved.MaxAttempts != 3 || resolved.DelaySeconds != 60 {
		t.Fatalf("enabled omissions not defaulted: %+v", resolved)
	}
	disabled := RetryPolicyOf(Config{Retry: &RetryPolicy{Enabled: false, MaxAttempts: 7, DelaySeconds: 900}})
	if disabled.MaxAttempts != 7 || disabled.DelaySeconds != 900 || disabled.Enabled {
		t.Fatalf("disabled policy rewritten: %+v", disabled)
	}
}

func TestNextRetryDelayAndRetryAllowedBounds(t *testing.T) {
	t.Parallel()
	if delay, ok := NextRetryDelay(Config{Retry: &RetryPolicy{Enabled: false}}); ok || delay != 0 {
		t.Fatalf("disabled policy offered a delay: %v %v", delay, ok)
	}
	c := Config{Retry: &RetryPolicy{Enabled: true, MaxAttempts: 2, DelaySeconds: 30}}
	if delay, ok := NextRetryDelay(c); !ok || delay != 30*time.Second {
		t.Fatalf("wrong delay: %v %v", delay, ok)
	}
	for _, tc := range []struct {
		attempt int
		allowed bool
	}{{0, false}, {1, true}, {2, true}, {3, false}, {-1, false}} {
		if allowed := RetryAllowed(c, tc.attempt); allowed != tc.allowed {
			t.Fatalf("attempt %d allowed=%v", tc.attempt, allowed)
		}
	}
}

func TestFailedStagesFiltersRefusalsAndDisabled(t *testing.T) {
	t.Parallel()
	r := Report{Checkpoint: &Checkpoint{Stages: []Stage{
		{ID: "error", Status: "error"},
		{ID: "processing", Status: "processing"},
		{ID: "queued", Status: "queued"},
		{ID: "skipped", Status: "skipped"},
		{ID: "disabled", Status: "disabled"},
		{ID: "success", Status: "success"},
		{ID: "partial", Status: "partial"},
	}}}
	failed := FailedStages(r)
	if len(failed) != 3 || failed[0] != "error" || failed[1] != "processing" || failed[2] != "queued" {
		t.Fatalf("wrong failed stages: %v", failed)
	}
	if FailedStages(Report{}) != nil {
		t.Fatal("missing checkpoint reported failures")
	}
}

func TestPrepareRepairResetsFailuresAndRecomputesCompleteness(t *testing.T) {
	t.Parallel()
	r := Report{Checkpoint: &Checkpoint{Stages: []Stage{
		{ID: "error", Status: "error", Attempted: true},
		{ID: "interrupted", Status: "processing", Attempted: true},
		{ID: "partial-evidence", Status: "partial", Attempted: true},
		{ID: "done", Status: "success", Complete: true},
		{ID: "legacy", Status: "success"},
	}}}
	PrepareRepair(&r)
	if r.Checkpoint.Complete {
		t.Fatal("partial evidence or legacy success must keep the pass incomplete")
	}
	for i, id := range []string{"error", "interrupted"} {
		stage := r.Checkpoint.Stages[i]
		if stage.ID != id || stage.Status != "queued" || stage.Attempted || stage.Complete {
			t.Fatalf("stage not reset: %+v", stage)
		}
	}
	if r.Checkpoint.Stages[2].Status != "partial" || r.Checkpoint.Stages[3].Complete != true {
		t.Fatalf("finished stages rewritten: %+v", r.Checkpoint.Stages)
	}
	complete := Report{Checkpoint: &Checkpoint{Stages: []Stage{{Status: "success", Complete: true}}}}
	PrepareRepair(&complete)
	if !complete.Checkpoint.Complete {
		t.Fatal("fully completed pass marked incomplete")
	}
}

func TestRetryPayloadsStayDistinctForDeduplication(t *testing.T) {
	t.Parallel()
	first, err := json.Marshal(ReportRetryPayload{ReportID: "r", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(ReportRetryPayload{ReportID: "r", Attempt: 2})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("chain payloads collapsed")
	}
}
