package iplookup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type enqueuedRetry struct {
	kind    string
	payload string
	at      time.Time
}

type reportRetryRepo struct {
	workerRepo
	gone     bool
	conflict bool
	enqueued []enqueuedRetry
}

func cloneRepairReport(r Report) (Report, error) {
	raw, err := EncodeReport(r)
	if err != nil {
		return Report{}, err
	}
	copy, err := DecodeReport(raw)
	copy.RepairRevision = r.RepairRevision
	return copy, err
}

func (r *reportRetryRepo) IPLookupReportByID(context.Context, string) (Report, Config, error) {
	if r.gone {
		return Report{}, Config{}, ErrRepairGone
	}
	if r.report.RepairRevision == "" {
		r.report.RepairRevision = "initial"
	}
	copy, err := cloneRepairReport(r.report)
	return copy, r.config, err
}

func (r *reportRetryRepo) UpdateIPLookupCachedReport(_ context.Context, report Report) (string, error) {
	if r.conflict || report.RepairRevision == "" || report.RepairRevision != r.report.RepairRevision {
		return "", ErrRepairConflict
	}
	copy, err := cloneRepairReport(report)
	if err != nil {
		return "", err
	}
	r.saves++
	copy.RepairRevision = fmt.Sprintf("revision-%d", r.saves)
	r.report = copy
	return copy.RepairRevision, nil
}

func (r *reportRetryRepo) EnqueueOutbox(_ context.Context, kind, payload string, at time.Time) error {
	r.enqueued = append(r.enqueued, enqueuedRetry{kind: kind, payload: payload, at: at})
	return nil
}

type liveRetryRepo struct {
	details  LiveDetails
	status   string
	gone     bool
	conflict bool
	updates  int
	enqueued []enqueuedRetry
}

func (r *liveRetryRepo) CachedIPDetails(context.Context, string) (*LiveDetails, error) {
	return nil, nil
}

func (r *liveRetryRepo) SaveIPDetails(_ context.Context, _ string, details LiveDetails, _ time.Time) (string, error) {
	r.details = details
	return "snapshot", nil
}

func cloneRepairLive(r LiveDetails) (LiveDetails, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return LiveDetails{}, err
	}
	var copy LiveDetails
	err = json.Unmarshal(raw, &copy)
	copy.SnapshotID, copy.RepairRevision = r.SnapshotID, r.RepairRevision
	return copy, err
}

func (r *liveRetryRepo) IPLiveSnapshotByID(context.Context, string) (LiveDetails, string, error) {
	if r.gone {
		return LiveDetails{}, "", ErrRepairGone
	}
	if r.details.RepairRevision == "" {
		r.details.RepairRevision = "initial"
	}
	copy, err := cloneRepairLive(r.details)
	return copy, r.status, err
}

func (r *liveRetryRepo) UpdateIPLiveSnapshot(_ context.Context, _ string, details LiveDetails) (string, error) {
	if r.conflict || details.RepairRevision == "" || details.RepairRevision != r.details.RepairRevision {
		return "", ErrRepairConflict
	}
	copy, err := cloneRepairLive(details)
	if err != nil {
		return "", err
	}
	r.updates++
	copy.RepairRevision = fmt.Sprintf("revision-%d", r.updates)
	r.details = copy
	return copy.RepairRevision, nil
}

func (r *liveRetryRepo) EnqueueOutbox(_ context.Context, kind, payload string, at time.Time) error {
	r.enqueued = append(r.enqueued, enqueuedRetry{kind: kind, payload: payload, at: at})
	return nil
}
