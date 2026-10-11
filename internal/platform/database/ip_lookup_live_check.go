package database

import (
	"context"
	"encoding/json"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func (s *Store) ipLookupCheckLive(ctx context.Context, check *iplookup.Check, raw, status string) error {
	var stage iplookup.LiveStage
	if err := json.Unmarshal([]byte(raw), &stage); err != nil {
		return err
	}
	if status == "processing" && stage.Status != "" {
		p := check.Progress
		if p.Completed == p.Total {
			p.Phase, p.CurrentProvider = "live", ""
		}
		p.Total++
		if stage.Status == "succeeded" || stage.Status == "partial" || stage.Status == "error" {
			p.Completed++
		}
	}
	if stage.SnapshotID == "" || status == "processing" {
		return nil
	}
	var snapshot string
	if err := s.db.QueryRowContext(ctx, `SELECT details_json FROM ip_lookup_live_snapshots WHERE id=?`, stage.SnapshotID).Scan(&snapshot); err != nil {
		return err
	}
	details, err := decodeIPDetails(snapshot, stage.SnapshotID, check.RequestedIP, "none")
	if err != nil {
		return err
	}
	check.LiveDetails = &details
	return nil
}
