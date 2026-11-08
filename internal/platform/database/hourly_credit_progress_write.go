package database

import (
	"context"
	"database/sql"
	"math"
	"sort"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type hourlyCreditDayKey struct {
	owner      string
	generation int64
	day        string
}

func applyHourlyCreditProgressTx(ctx context.Context, tx *sql.Tx, previous map[string]model.HourlyCreditReservation, values []model.HourlyCreditReservation) error {
	if len(values) == 0 {
		return nil
	}
	days := map[hourlyCreditDayKey][2]int64{}
	for _, r := range values {
		old := previous[r.ReservationID]
		delta := r.AdmittedUploadBytes + r.AdmittedDownloadBytes - old.AdmittedUploadBytes - old.AdmittedDownloadBytes
		key := hourlyCreditDayKey{r.OwnerID, r.AccountGeneration, r.DayUTC}
		totals := days[key]
		kind := 0
		if r.Kind == "extra" {
			kind = 1
		}
		if delta > math.MaxInt64-totals[kind] {
			return ErrConflict
		}
		totals[kind] += delta
		days[key] = totals
	}
	keys := make([]hourlyCreditDayKey, 0, len(days))
	for key := range days {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.owner != b.owner {
			return a.owner < b.owner
		}
		if a.generation != b.generation {
			return a.generation < b.generation
		}
		return a.day < b.day
	})
	for _, key := range keys {
		day, err := hourlyCreditDayTx(ctx, tx, key.owner, key.generation, key.day)
		if err != nil {
			return err
		}
		move := days[key]
		if move[0] > day.FreeReservedBytes || move[1] > day.ExtraReservedBytes {
			return ErrConflict
		}
		for i, kind := range []string{"free", "extra"} {
			r := model.HourlyCreditRequest{OwnerID: key.owner, AccountGeneration: key.generation, DayUTC: key.day, Kind: kind}
			if err = adjustHourlyCreditDayTx(ctx, tx, r, move[i], -move[i]); err != nil {
				return err
			}
		}
	}
	ids := make([]string, 0, len(values))
	up, down, unknown := make([]int64, 0, len(values)), make([]int64, 0, len(values)), make([]int64, 0, len(values))
	for _, r := range values {
		ids = append(ids, r.ReservationID)
		up, down, unknown = append(up, r.AdmittedUploadBytes), append(down, r.AdmittedDownloadBytes), append(unknown, r.UnknownBytes)
	}
	result, err := tx.ExecContext(ctx, `UPDATE hourly_credit_reservations r SET admitted_upload_bytes=v.up_bytes,admitted_download_bytes=v.down_bytes,unknown_bytes=v.unknown_bytes FROM unnest($1::text[],$2::bigint[],$3::bigint[],$4::bigint[]) AS v(id,up_bytes,down_bytes,unknown_bytes) WHERE r.reservation_id=v.id`, ids, up, down, unknown)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != int64(len(values)) {
		return ErrConflict
	}
	return err
}
