package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func handleAbuseIPBan(ctx context.Context, recordID string, item database.AbuseJob, store *database.Store, remna remnaAdapter) error {
	scanID, err := store.AbuseIPBanScan(ctx, recordID)
	if errors.Is(err, database.ErrNotFound) {
		scanID, err = remna.StartAbuseIPBanScan(ctx, item.RemoteUserID)
		if err != nil {
			return err
		}
		if err = store.SaveAbuseIPBanScan(ctx, recordID, scanID, time.Now().UTC()); err != nil {
			return err
		}
		return errors.New("abuse connection scan is pending")
	}
	if err != nil {
		return err
	}
	completed, err := remna.CompleteAbuseIPBan(ctx, scanID, item.Nodes, item.AllNodes, item.DurationMinutes*60)
	if err != nil {
		return err
	}
	if !completed {
		return errors.New("abuse connection scan is pending")
	}
	return nil
}
