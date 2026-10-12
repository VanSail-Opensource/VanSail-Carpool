package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

// scheduleIPLookupReportRetryTx queues the first repair attempt when a freshly
// frozen report is partial with failed stages and the current policy allows
// background repair. The payload attempt counter keeps the chain deduplicated.
func scheduleIPLookupReportRetryTx(ctx context.Context, tx *sql.Tx, id string, report iplookup.Report, now time.Time) error {
	if report.Status != "partial" || len(iplookup.FailedStages(report)) == 0 {
		return nil
	}
	c, err := ipConfigTx(ctx, tx)
	if err != nil {
		return err
	}
	delay, ok := iplookup.NextRetryDelay(c)
	if !ok {
		return nil
	}
	payload, err := json.Marshal(iplookup.ReportRetryPayload{ReportID: id, Attempt: 1})
	if err != nil {
		return err
	}
	_, err = insertOutboxJobTx(ctx, tx, iplookup.ReportRetryOutboxKind, string(payload), now.Add(delay), now)
	return err
}

// scheduleIPLiveRetryTx queues the first section repair attempt for a freshly
// stored partial snapshot.
func scheduleIPLiveRetryTx(ctx context.Context, tx *sql.Tx, id string, delay time.Duration, now time.Time) error {
	payload, err := json.Marshal(iplookup.LiveRetryPayload{SnapshotID: id, Attempt: 1})
	if err != nil {
		return err
	}
	_, err = insertOutboxJobTx(ctx, tx, iplookup.LiveRetryOutboxKind, string(payload), now.Add(delay), now)
	return err
}

// ipLiveRetryDelay resolves the configured repair wait from current settings.
// Unreadable or missing settings keep the enabled defaults so repair survives
// configuration glitches.
func (s *Store) ipLiveRetryDelay(ctx context.Context) (time.Duration, bool) {
	raw := ""
	if setting, err := s.GetSetting(ctx, iplookup.SettingKey); err == nil {
		raw = setting.Value
	}
	c, err := iplookup.DecodeConfig(raw)
	if err != nil {
		c = iplookup.DefaultConfig()
	}
	return iplookup.NextRetryDelay(c)
}

// IPLookupReportByID loads one report row with current saved settings.
func (s *Store) IPLookupReportByID(ctx context.Context, id string) (iplookup.Report, iplookup.Config, error) {
	var raw, configJSON string
	err := s.db.QueryRowContext(ctx, `SELECT report_json,COALESCE((SELECT value FROM settings WHERE key=?),'') FROM ip_lookup_reports WHERE id=?`, iplookup.SettingKey, id).Scan(&raw, &configJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return iplookup.Report{}, iplookup.Config{}, iplookup.ErrRepairGone
	}
	if err != nil {
		return iplookup.Report{}, iplookup.Config{}, err
	}
	report, err := iplookup.DecodeReport([]byte(raw))
	if err != nil {
		return iplookup.Report{}, iplookup.Config{}, err
	}
	config, err := iplookup.DecodeConfig(configJSON)
	return report, config, err
}

// UpdateIPLookupCachedReport repairs one frozen partial report in place. The
// original completion timestamp stays untouched so newest-wins cache ordering
// keeps preferring evidence from later paid refreshes.
func (s *Store) UpdateIPLookupCachedReport(ctx context.Context, report iplookup.Report) error {
	raw, err := iplookup.EncodeReport(report)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE ip_lookup_reports SET status=?,report_json=? WHERE id=? AND status='partial'`, report.Status, string(raw), report.ID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected != 1 {
		return iplookup.ErrRepairConflict
	}
	return nil
}

// IPLiveSnapshotByID loads one stored snapshot with its freeze status.
func (s *Store) IPLiveSnapshotByID(ctx context.Context, id string) (iplookup.LiveDetails, string, error) {
	var raw, status string
	err := s.db.QueryRowContext(ctx, `SELECT details_json,status FROM ip_lookup_live_snapshots WHERE id=?`, id).Scan(&raw, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return iplookup.LiveDetails{}, "", iplookup.ErrRepairGone
	}
	if err != nil {
		return iplookup.LiveDetails{}, "", err
	}
	var details iplookup.LiveDetails
	if err := json.Unmarshal([]byte(raw), &details); err != nil {
		return iplookup.LiveDetails{}, "", err
	}
	details = iplookup.CompactLiveDetails(details)
	details.SnapshotID = id
	return details, status, nil
}

// UpdateIPLiveSnapshot repairs failed sections of one frozen partial snapshot
// in place without advancing its cache ordering position.
func (s *Store) UpdateIPLiveSnapshot(ctx context.Context, id string, details iplookup.LiveDetails) error {
	details = iplookup.CompactLiveDetails(details)
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if len(raw) > 4*1024*1024 {
		return &iplookup.CodeError{Code: "IP_DETAILS_INVALID_RESPONSE"}
	}
	status := "succeeded"
	if iplookup.LivePartial(details) {
		status = "partial"
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE ip_lookup_live_snapshots SET details_json=?,status=? WHERE id=? AND status='partial'`, string(raw), status, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected != 1 {
		return iplookup.ErrRepairConflict
	}
	return nil
}
