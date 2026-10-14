package database

import (
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func scanRollover(row rowScanner) (model.PurchaseRollover, error) {
	var value model.PurchaseRollover
	var allocated, used, eligible, remaining sql.NullInt64
	var created, updated string
	var completed sql.NullString
	if err := row.Scan(&value.PurchaseID, &value.Status, &value.TrafficLimitBytes, &allocated, &used, &eligible, &remaining, &value.MinimumRemainingBPS,
		&value.NetPaidTXBMinor, &value.CreditedTXBMinor, &value.ExceptionCode, &value.Attempts, &created, &updated, &completed, &value.AlgorithmVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PurchaseRollover{}, ErrNotFound
		}
		return model.PurchaseRollover{}, err
	}
	if used.Valid {
		value.UsedTrafficBytes = &used.Int64
	}
	if allocated.Valid {
		value.AllocatedBytes = &allocated.Int64
	}
	if eligible.Valid {
		value.EligibleUnusedBytes = &eligible.Int64
	}
	if remaining.Valid {
		value.RemainingBytes = &remaining.Int64
	}
	var err error
	if value.CreatedAt, err = parseStamp(created); err != nil {
		return model.PurchaseRollover{}, err
	}
	if value.UpdatedAt, err = parseStamp(updated); err != nil {
		return model.PurchaseRollover{}, err
	}
	if completed.Valid {
		parsed, parseErr := parseStamp(completed.String)
		if parseErr != nil {
			return model.PurchaseRollover{}, parseErr
		}
		value.CompletedAt = &parsed
	}
	return value, nil
}
